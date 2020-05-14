package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/pubsub"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/pkg/errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// VerifyPreSignShardBlock Verify Shard Block Before Signing
// Used for PBFT consensus
// this block doesn't have full information (incomplete block)
func (blockchain *BlockChain) VerifyPreSignShardBlock(shardBlock *ShardBlock, shardID byte) error {
	//get view that block link to
	preHash := shardBlock.Header.PreviousBlockHash
	view := blockchain.ShardChain[int(shardID)].GetViewByHash(preHash)
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if view == nil {
		blockchain.config.Syncker.SyncMissingShardBlock(ctx, "", shardID, preHash)
	}
	var checkShardUntilTimeout = func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return errors.New(fmt.Sprintf("ShardBlock %v link to wrong view (%s)", shardBlock.GetHeight(), preHash.String()))
			default:
				if blockchain.ShardChain[shardID].GetViewByHash(preHash) != nil {
					return nil
				}
				time.Sleep(time.Second)
			}
		}
	}
	if err := checkShardUntilTimeout(ctx); err != nil {
		return err
	}

	curView := view.(*ShardBestState)
	Logger.log.Infof("SHARD %+v | Verify ShardBlock for signing process %d, with hash %+v", shardID, shardBlock.Header.Height, *shardBlock.Hash())

	// fetch beacon blocks
	previousBeaconHeight := curView.BeaconHeight
	var checkBeaconUntilTimeout = func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return errors.New("Wait for beacon timeout")
			default:
				if shardBlock.Header.BeaconHeight <= blockchain.BeaconChain.GetFinalView().GetHeight() {
					return nil
				}
				time.Sleep(time.Second)
			}
		}
	}
	ctx, _ = context.WithTimeout(context.Background(), time.Second*5)
	if checkBeaconUntilTimeout(ctx) != nil {
		return errors.New(fmt.Sprintf("Beacon %d not ready, latest is %d", shardBlock.Header.BeaconHeight, blockchain.GetBeaconBestState().BeaconHeight))
	}

	beaconBlocks, err := FetchBeaconBlockFromHeight(blockchain.GetBeaconChainDatabase(), previousBeaconHeight+1, shardBlock.Header.BeaconHeight)
	if err != nil {
		return err
	}

	//========Verify shardBlock only
	if err := blockchain.verifyPreProcessingShardBlock(curView, shardBlock, beaconBlocks, shardID, true); err != nil {
		return err
	}
	//========Verify shardBlock with previous best state

	// Verify shardBlock with previous best state
	// DO NOT verify agg signature in this function
	if err := curView.verifyBestStateWithShardBlock(blockchain, shardBlock, false, shardID); err != nil {
		return err
	}
	//========updateShardBestState best state with new shardBlock
	newBeststate, err := curView.updateShardBestState(blockchain, shardBlock, beaconBlocks, newCommitteeChange())
	if err != nil {
		return err
	}
	//========Post verififcation: verify new beaconstate with corresponding shardBlock
	if err := newBeststate.verifyPostProcessingShardBlock(shardBlock, shardID); err != nil {
		return err
	}
	Logger.log.Infof("SHARD %+v | Block %d, with hash %+v is VALID for 🖋 signing", shardID, shardBlock.GetHeight(), shardBlock.Hash().String())
	return nil
}

// InsertShardBlock Insert Shard Block into blockchain
// this block must have full information (complete block)
func (blockchain *BlockChain) InsertShardBlock(shardBlock *ShardBlock, shouldValidate bool) error {
	blockHash := shardBlock.Header.Hash()
	blockHeight := shardBlock.Header.Height
	shardID := shardBlock.Header.ShardID

	Logger.log.Infof("SHARD %+v | InsertShardBlock %+v with hash %+v \n", shardID, blockHeight, blockHash)
	blockchain.ShardChain[int(shardID)].insertLock.Lock()
	defer blockchain.ShardChain[int(shardID)].insertLock.Unlock()
	//startTimeInsertShardBlock := time.Now()
	committeeChange := newCommitteeChange()

	//check if view is committed
	checkView := blockchain.ShardChain[int(shardID)].GetViewByHash(blockHash)
	if checkView != nil {
		return nil
	}

	//get view that block link to
	preHash := shardBlock.Header.PreviousBlockHash
	preView := blockchain.ShardChain[int(shardID)].GetViewByHash(preHash)
	if preView == nil {
		return NewBlockChainError(InsertShardBlockError, fmt.Errorf("ShardBlock %v link to wrong view (%s)", blockHeight, preHash.String()))
	}
	curView := preView.(*ShardBestState)

	if blockHeight != curView.ShardHeight+1 {
		return NewBlockChainError(InsertShardBlockError, fmt.Errorf("Not expected height, current view height %+v, incomming block height %+v", curView.ShardHeight, blockHeight))
	}
	// fetch beacon blocks
	previousBeaconHeight := curView.BeaconHeight
	beaconBlocks, err := FetchBeaconBlockFromHeight(blockchain.GetBeaconChainDatabase(), previousBeaconHeight+1, shardBlock.Header.BeaconHeight)
	if err != nil {
		return NewBlockChainError(FetchBeaconBlocksError, err)
	}
	if shouldValidate {
		Logger.log.Infof("SHARD %+v | Verify Pre Processing, block height %+v with hash %+vt \n", shardID, blockHeight, blockHash)
		if err := blockchain.verifyPreProcessingShardBlock(curView, shardBlock, beaconBlocks, shardID, false); err != nil {
			return err
		}
	} else {
		Logger.log.Infof("SHARD %+v | SKIP Verify Pre Processing, block height %+v with hash %+v \n", shardID, blockHeight, blockHash)
	}
	// Verify block with previous best state
	Logger.log.Debugf("SHARD %+v | Verify BestState With Shard Block, block height %+v with hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, blockHash)
	if err := curView.verifyBestStateWithShardBlock(blockchain, shardBlock, true, shardID); err != nil {
		return err
	}
	if err := blockchain.config.ConsensusEngine.ValidateBlockCommitteSig(shardBlock, curView.ShardCommittee); err != nil {
		return err
	}
	Logger.log.Debugf("SHARD %+v | BackupCurrentShardState, block height %+v with hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, blockHash)

	oldCommittee, err := incognitokey.CommitteeKeyListToString(curView.ShardCommittee)
	if err != nil {
		return err
	}

	Logger.log.Debugf("SHARD %+v | Update ShardBestState, block height %+v with hash %+v \n", shardBlock.Header.ShardID, shardBlock.Header.Height, blockHash)
	newBestState, err := curView.updateShardBestState(blockchain, shardBlock, beaconBlocks, newCommitteeChange())
	if err != nil {
		return err
	}

	Logger.log.Infof("SHARD %+v | Update NumOfBlocksByProducers, block height %+v with hash %+v \n", shardID, blockHeight, blockHash)
	// update number of blocks produced by producers to shard best state
	newBestState.updateNumOfBlocksByProducers(shardBlock)

	newCommittee, err := incognitokey.CommitteeKeyListToString(newBestState.ShardCommittee)
	if err != nil {
		return err
	}
	if !common.CompareStringArray(oldCommittee, newCommittee) {
		go blockchain.config.ConsensusEngine.CommitteeChange(common.GetShardChainKey(shardID))
	}
	//========Post verification: verify new beaconstate with corresponding block
	//if shouldValidate {
	Logger.log.Debugf("SHARD %+v | Verify Post Processing, block height %+v with hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, blockHash)
	if err := newBestState.verifyPostProcessingShardBlock(shardBlock, shardID); err != nil {
		fmt.Println("Instructions", shardBlock.Body.Instructions)
		return err
	}
	//} else {
	//	Logger.log.Infof("SHARD %+v | SKIP Verify Post Processing, block height %+v with hash %+v \n", shardID, blockHeight, blockHash)
	//}

	Logger.log.Infof("SHARD %+v | Update Beacon Instruction, block height %+v with hash %+v \n", shardID, blockHeight, blockHash)
	err = blockchain.processSalaryInstructions(newBestState.rewardStateDB, beaconBlocks, shardID)
	if err != nil {
		return err
	}
	Logger.log.Infof("SHARD %+v | Store New Shard Block And Update Data, block height %+v with hash %+v \n", shardID, blockHeight, blockHash)
	//========Store new  Shard block and new shard bestState
	confirmBeaconBlock := NewBeaconBlock()
	if len(beaconBlocks) > 0 {
		confirmBeaconBlock = beaconBlocks[len(beaconBlocks)-1]
	} else {
		confirmBeaconBlocks, err := blockchain.GetBeaconBlockByHeight(shardBlock.Header.BeaconHeight)
		if err != nil {
			return err
		}
		confirmBeaconBlock = confirmBeaconBlocks[0]
	}
	err = blockchain.processStoreShardBlock(newBestState, shardBlock, committeeChange, confirmBeaconBlock)
	if err != nil {

		return err
	}
	blockchain.removeOldDataAfterProcessingShardBlock(shardBlock, shardID)
	go blockchain.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.NewShardblockTopic, shardBlock))
	go blockchain.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.ShardBeststateTopic, newBestState))
	Logger.log.Infof("SHARD %+v | Finish Insert new block %d, with hash %+v 🔗", shardBlock.Header.ShardID, shardBlock.Header.Height, blockHash)
	return nil
}

// updateNumOfBlocksByProducers updates number of blocks produced by producers to shard best state
func (shardBestState *ShardBestState) updateNumOfBlocksByProducers(shardBlock *ShardBlock) {
	isSwapInstContained := false
	for _, inst := range shardBlock.Body.Instructions {
		if len(inst) > 0 && inst[0] == SwapAction {
			isSwapInstContained = true
			break
		}
	}
	producer := shardBlock.GetProducerPubKeyStr()
	if isSwapInstContained {
		// reset number of blocks produced by producers
		shardBestState.NumOfBlocksByProducers = map[string]uint64{
			producer: 1,
		}
	} else {
		// Update number of blocks produced by producers in epoch
		numOfBlks, found := shardBestState.NumOfBlocksByProducers[producer]
		if !found {
			shardBestState.NumOfBlocksByProducers[producer] = 1
		} else {
			shardBestState.NumOfBlocksByProducers[producer] = numOfBlks + 1
		}
	}
}

// verifyPreProcessingShardBlock DOES NOT verify new block with best state
// DO NOT USE THIS with GENESIS BLOCK
// Verification condition:
//	- Producer Address is not empty
//	- ShardID: of received block same shardID
//	- Version: shard block version is one of pre-defined versions
//	- Parent (previous) block must be found in database ( current block point to an exist block in database )
//	- Height: parent block height + 1
//	- Epoch: blockHeight % Epoch ? Parent Epoch + 1 : Current Epoch
//	- Timestamp: block timestamp must be greater than previous block timestamp
//	- TransactionRoot: rebuild transaction root from txs in block and compare with transaction root in header
//	- ShardTxRoot: rebuild shard transaction root from txs in block and compare with shard transaction root in header
//	- CrossOutputCoinRoot: rebuild cross shard output root from cross output coin in block and compare with cross shard output coin
//		+ cross output coin must be re-created (from cross shard block) if verify block for signing
//	- InstructionRoot: rebuild instruction root from instructions and txs in block and compare with instruction root in header
//		+ instructions must be re-created (from beacon block and instruction) if verify block for signing
//	- InstructionMerkleRoot: rebuild instruction root from instructions and txs in block and compare with instruction root in header
//	- TotalTxFee: calculate tx token fee from all transaction in block then compare with header
//	- CrossShars: Verify Cross Shard Bitmap
//	- BeaconHeight: fetch beacon hash using beacon height in current shard block
//	- BeaconHash: compare beacon hash in database with beacon hash in shard block
//	- Verify swap instruction
//	- Validate transaction created from miner via instruction
//	- Validate Response Transaction From Transaction with Metadata
//	- ALL Transaction in block: see in verifyTransactionFromNewBlock
func (blockchain *BlockChain) verifyPreProcessingShardBlock(curView *ShardBestState, shardBlock *ShardBlock, beaconBlocks []*BeaconBlock, shardID byte, isPreSign bool) error {
	startTimeVerifyPreProcessingShardBlock := time.Now()
	Logger.log.Debugf("SHARD %+v | Begin verifyPreProcessingShardBlock Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	if shardBlock.Header.ShardID != shardID {
		return NewBlockChainError(WrongShardIDError, fmt.Errorf("Expect receive shardBlock from Shard ID %+v but get %+v", shardID, shardBlock.Header.ShardID))
	}

	if shardBlock.Header.Height > curView.ShardHeight+1 {
		return NewBlockChainError(WrongBlockHeightError, fmt.Errorf("Expect shardBlock height %+v but get %+v", curView.ShardHeight+1, shardBlock.Header.Height))
	}
	// Verify parent hash exist or not
	previousBlockHash := shardBlock.Header.PreviousBlockHash
	previousShardBlockData, err := rawdbv2.GetShardBlockByHash(blockchain.GetShardChainDatabase(shardID), previousBlockHash)
	if err != nil {
		return NewBlockChainError(FetchPreviousBlockError, err)
	}

	previousShardBlock := ShardBlock{}
	err = json.Unmarshal(previousShardBlockData, &previousShardBlock)
	if err != nil {
		return NewBlockChainError(UnmashallJsonShardBlockError, err)
	}
	// Verify shardBlock height with parent shardBlock
	if previousShardBlock.Header.Height+1 != shardBlock.Header.Height {
		return NewBlockChainError(WrongBlockHeightError, fmt.Errorf("Expect receive shardBlock height %+v but get %+v", previousShardBlock.Header.Height+1, shardBlock.Header.Height))
	}
	// Verify timestamp with parent shardBlock
	if shardBlock.Header.Timestamp <= previousShardBlock.Header.Timestamp {
		return NewBlockChainError(WrongTimestampError, fmt.Errorf("Expect receive shardBlock has timestamp must be greater than %+v but get %+v", previousShardBlock.Header.Timestamp, shardBlock.Header.Timestamp))
	}
	// Verify transaction root
	txMerkleTree := Merkle{}.BuildMerkleTreeStore(shardBlock.Body.Transactions)
	txRoot := &common.Hash{}
	if len(txMerkleTree) > 0 {
		txRoot = txMerkleTree[len(txMerkleTree)-1]
	}
	if !bytes.Equal(shardBlock.Header.TxRoot.GetBytes(), txRoot.GetBytes()) && shardBlock.Header.Height != 487260 && shardBlock.Header.Height != 487261 {
		return NewBlockChainError(TransactionRootHashError, fmt.Errorf("Expect transaction root hash %+v but get %+v", shardBlock.Header.TxRoot, txRoot))
	}
	// Verify ShardTx Root
	_, shardTxMerkleData := CreateShardTxRoot(shardBlock.Body.Transactions)
	shardTxRoot := shardTxMerkleData[len(shardTxMerkleData)-1]
	if !bytes.Equal(shardBlock.Header.ShardTxRoot.GetBytes(), shardTxRoot.GetBytes()) {
		return NewBlockChainError(ShardTransactionRootHashError, fmt.Errorf("Expect shard transaction root hash %+v but get %+v", shardBlock.Header.ShardTxRoot, shardTxRoot))
	}
	// Verify crossTransaction coin
	if !VerifyMerkleCrossTransaction(shardBlock.Body.CrossTransactions, shardBlock.Header.CrossTransactionRoot) {
		return NewBlockChainError(CrossShardTransactionRootHashError, fmt.Errorf("Expect cross shard transaction root hash %+v", shardBlock.Header.CrossTransactionRoot))
	}
	// Verify Action
	txInstructions, err := CreateShardInstructionsFromTransactionAndInstruction(shardBlock.Body.Transactions, blockchain, shardID)
	if err != nil {
		Logger.log.Error(err)
		return NewBlockChainError(ShardIntructionFromTransactionAndInstructionError, err)
	}
	if !isPreSign {
		totalInstructions := []string{}
		for _, value := range txInstructions {
			totalInstructions = append(totalInstructions, value...)
		}
		for _, value := range shardBlock.Body.Instructions {
			totalInstructions = append(totalInstructions, value...)
		}
		if hash, ok := verifyHashFromStringArray(totalInstructions, shardBlock.Header.InstructionsRoot); !ok {
			return NewBlockChainError(InstructionsHashError, fmt.Errorf("Expect instruction hash to be %+v but get %+v at block %+v hash %+v", shardBlock.Header.InstructionsRoot, hash, shardBlock.Header.Height, shardBlock.Hash().String()))
		}
	}
	totalTxsFee := make(map[common.Hash]uint64)
	for _, tx := range shardBlock.Body.Transactions {
		totalTxsFee[*tx.GetTokenID()] += tx.GetTxFee()
		txType := tx.GetType()
		if txType == common.TxCustomTokenPrivacyType {
			txCustomPrivacy := tx.(*transaction.TxCustomTokenPrivacy)
			totalTxsFee[*txCustomPrivacy.GetTokenID()] = txCustomPrivacy.GetTxFeeToken()
		}
	}
	tokenIDsfromTxs := make([]common.Hash, 0)
	for tokenID := range totalTxsFee {
		tokenIDsfromTxs = append(tokenIDsfromTxs, tokenID)
	}
	sort.Slice(tokenIDsfromTxs, func(i int, j int) bool {
		res, _ := tokenIDsfromTxs[i].Cmp(&tokenIDsfromTxs[j])
		return res == -1
	})
	tokenIDsfromBlock := make([]common.Hash, 0)
	for tokenID := range shardBlock.Header.TotalTxsFee {
		tokenIDsfromBlock = append(tokenIDsfromBlock, tokenID)
	}
	sort.Slice(tokenIDsfromBlock, func(i int, j int) bool {
		res, _ := tokenIDsfromBlock[i].Cmp(&tokenIDsfromBlock[j])
		return res == -1
	})
	if len(tokenIDsfromTxs) != len(tokenIDsfromBlock) {
		return NewBlockChainError(WrongBlockTotalFeeError, fmt.Errorf("Expect Total Fee to be equal, From Txs %+v, From Block %+v", len(tokenIDsfromTxs), len(tokenIDsfromBlock)))
	}
	for i, tokenID := range tokenIDsfromTxs {
		if !tokenIDsfromTxs[i].IsEqual(&tokenIDsfromBlock[i]) {
			return NewBlockChainError(WrongBlockTotalFeeError, fmt.Errorf("Expect Total Fee to be equal, From Txs %+v, From Block %+v", tokenIDsfromTxs[i], tokenIDsfromBlock[i]))
		}
		if totalTxsFee[tokenID] != shardBlock.Header.TotalTxsFee[tokenID] {
			return NewBlockChainError(WrongBlockTotalFeeError, fmt.Errorf("Expect Total Fee to be equal, From Txs %+v, From Block Header %+v", totalTxsFee[tokenID], shardBlock.Header.TotalTxsFee[tokenID]))
		}
	}
	// Verify Cross Shards
	crossShards := CreateCrossShardByteArray(shardBlock.Body.Transactions, shardID)
	if len(crossShards) != len(shardBlock.Header.CrossShardBitMap) {
		return NewBlockChainError(CrossShardBitMapError, fmt.Errorf("Expect number of cross shardID is %+v but get %+v", len(shardBlock.Header.CrossShardBitMap), len(crossShards)))
	}
	for index := range crossShards {
		if crossShards[index] != shardBlock.Header.CrossShardBitMap[index] {
			return NewBlockChainError(CrossShardBitMapError, fmt.Errorf("Expect Cross Shard Bitmap of shardID %+v is %+v but get %+v", index, shardBlock.Header.CrossShardBitMap[index], crossShards[index]))
		}
	}
	// Check if InstructionMerkleRoot is the root of merkle tree containing all instructions in this shardBlock
	flattenTxInsts, err := FlattenAndConvertStringInst(txInstructions)
	if err != nil {
		return NewBlockChainError(FlattenAndConvertStringInstError, fmt.Errorf("Instruction from Tx: %+v", err))
	}
	flattenInsts, err := FlattenAndConvertStringInst(shardBlock.Body.Instructions)
	if err != nil {
		return NewBlockChainError(FlattenAndConvertStringInstError, fmt.Errorf("Instruction from shardBlock body: %+v", err))
	}
	insts := append(flattenTxInsts, flattenInsts...) // Order of instructions must be the same as when creating new shard shardBlock
	root := GetKeccak256MerkleRoot(insts)
	if !bytes.Equal(root, shardBlock.Header.InstructionMerkleRoot[:]) {
		return NewBlockChainError(InstructionMerkleRootError, fmt.Errorf("Expect transaction merkle root to be %+v but get %+v", shardBlock.Header.InstructionMerkleRoot, string(root)))
	}
	//Get beacon hash by height in db
	//If hash not found then fail to verify
	beaconHashs, err := rawdbv2.GetBeaconBlockHashByIndex(blockchain.GetBeaconChainDatabase(), shardBlock.Header.BeaconHeight)
	if err != nil {
		return NewBlockChainError(FetchBeaconBlockHashError, err)
	}
	beaconHash := beaconHashs[0]
	//Hash in db must be equal to hash in shard shardBlock
	newHash, err := common.Hash{}.NewHash(shardBlock.Header.BeaconHash.GetBytes())
	if err != nil {
		return NewBlockChainError(HashError, err)
	}
	if !newHash.IsEqual(&beaconHash) {
		return NewBlockChainError(BeaconBlockNotCompatibleError, fmt.Errorf("Expect beacon shardBlock hash to be %+v but get %+v", beaconHash, newHash))
	}
	// Swap instruction
	for _, l := range shardBlock.Body.Instructions {
		if l[0] == "swap" {
			if l[3] != "shard" || l[4] != strconv.Itoa(int(shardID)) {
				return NewBlockChainError(SwapInstructionError, fmt.Errorf("invalid swap instruction %+v", l))
			}
		}
	}
	// Verify response transactions
	instsForValidations := [][]string{}
	instsForValidations = append(instsForValidations, shardBlock.Body.Instructions...)
	for _, beaconBlock := range beaconBlocks {
		instsForValidations = append(instsForValidations, beaconBlock.Body.Instructions...)
	}
	invalidTxs, err := blockchain.verifyMinerCreatedTxBeforeGettingInBlock(instsForValidations, shardBlock.Body.Transactions, shardID)
	if err != nil {
		return NewBlockChainError(TransactionCreatedByMinerError, err)
	}
	if len(invalidTxs) > 0 {
		return NewBlockChainError(TransactionCreatedByMinerError, fmt.Errorf("There are %d invalid txs", len(invalidTxs)))
	}
	err = blockchain.ValidateResponseTransactionFromTxsWithMetadata(shardBlock)
	if err != nil {
		return NewBlockChainError(ResponsedTransactionWithMetadataError, err)
	}
	shardVerifyPreprocesingTimer.UpdateSince(startTimeVerifyPreProcessingShardBlock)
	// Get cross shard shardBlock from pool
	if isPreSign {
		err := blockchain.verifyPreProcessingShardBlockForSigning(curView, shardBlock, beaconBlocks, txInstructions, shardID)
		if err != nil {
			return err
		}
	}
	Logger.log.Debugf("SHARD %+v | Finish verifyPreProcessingShardBlock Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	return err
}

// VerifyPreProcessingShardBlockForSigning verify shard block before a validator signs new shard block
//	- Verify Transactions In New Block
//	- Generate Instruction (from beacon), create instruction root and compare instruction root with instruction root in header
//	- Get Cross Output Data from cross shard block (shard pool) and verify cross transaction hash
//	- Get Cross Tx Custom Token from cross shard block (shard pool) then verify
//
func (blockchain *BlockChain) verifyPreProcessingShardBlockForSigning(curView *ShardBestState, shardBlock *ShardBlock, beaconBlocks []*BeaconBlock, txInstructions [][]string, shardID byte) error {
	var err error
	var isOldBeaconHeight = false
	startTimeVerifyPreProcessingShardBlockForSigning := time.Now()
	// Verify Transaction
	//get beacon height from shard block
	beaconHeight := shardBlock.Header.BeaconHeight
	if err := blockchain.verifyTransactionFromNewBlock(shardID, shardBlock.Body.Transactions, int64(beaconHeight), curView); err != nil {
		return NewBlockChainError(TransactionFromNewBlockError, err)
	}
	// Verify Instruction
	instructions := [][]string{}
	shardCommittee, err := incognitokey.CommitteeKeyListToString(curView.ShardCommittee)
	if err != nil {
		return err
	}
	shardPendingValidator, _, _ := blockchain.processInstructionFromBeacon(curView, beaconBlocks, shardID, newCommitteeChange())
	if curView.BeaconHeight == shardBlock.Header.BeaconHeight {
		isOldBeaconHeight = true
	}
	instructions, shardPendingValidator, shardCommittee, err = blockchain.generateInstruction(curView, shardID, shardBlock.Header.BeaconHeight, isOldBeaconHeight, beaconBlocks, shardPendingValidator, shardCommittee)
	if err != nil {
		return NewBlockChainError(GenerateInstructionError, err)
	}
	totalInstructions := []string{}
	for _, value := range txInstructions {
		totalInstructions = append(totalInstructions, value...)
	}
	for _, value := range instructions {
		totalInstructions = append(totalInstructions, value...)
	}
	if hash, ok := verifyHashFromStringArray(totalInstructions, shardBlock.Header.InstructionsRoot); !ok {
		return NewBlockChainError(InstructionsHashError, fmt.Errorf("Expect instruction hash to be %+v but %+v", shardBlock.Header.InstructionsRoot, hash))
	}
	toShard := shardID
	var toShardAllCrossShardBlock = make(map[byte][]*CrossShardBlock)

	// blockchain.config.Syncker.GetCrossShardBlocksForShardValidator(toShard, list map[byte]common.Hash) map[byte][]interface{}
	crossShardRequired := make(map[byte][]uint64)
	for fromShard, crossTransactions := range shardBlock.Body.CrossTransactions {
		for _, crossTransaction := range crossTransactions {
			//fmt.Println("Crossshard from ", fromShard, crossTransaction.BlockHash)
			crossShardRequired[fromShard] = append(crossShardRequired[fromShard], crossTransaction.BlockHeight)
		}
	}
	crossShardBlksFromPool, err := blockchain.config.Syncker.GetCrossShardBlocksForShardValidator(toShard, crossShardRequired)
	if err != nil {
		return NewBlockChainError(CrossShardBlockError, fmt.Errorf("Unable to get required crossShard blocks from pool in time"))
	}
	for sid, v := range crossShardBlksFromPool {
		for _, b := range v {
			toShardAllCrossShardBlock[sid] = append(toShardAllCrossShardBlock[sid], b.(*CrossShardBlock))
		}
	}
	for fromShard, crossTransactions := range shardBlock.Body.CrossTransactions {
		toShardCrossShardBlocks := toShardAllCrossShardBlock[fromShard]
		// if !ok {

		// 	//blockchain.Synker.SyncBlkCrossShard(false, false, []common.Hash{}, heights, fromShard, shardID, "")
		// 	return NewBlockChainError(CrossShardBlockError, fmt.Errorf("Cross Shard Block From Shard %+v Not Found in Pool", fromShard))
		// }
		sort.SliceStable(toShardCrossShardBlocks[:], func(i, j int) bool {
			return toShardCrossShardBlocks[i].Header.Height < toShardCrossShardBlocks[j].Header.Height
		})
		//startHeight := blockchain.GetBestStateShard(toShard).BestCrossShard[fromShard]
		isValids := 0
		for _, crossTransaction := range crossTransactions {
			for index, toShardCrossShardBlock := range toShardCrossShardBlocks {
				//Compare block height and block hash
				if crossTransaction.BlockHeight == toShardCrossShardBlock.Header.Height {
					compareCrossTransaction := CrossTransaction{
						TokenPrivacyData: toShardCrossShardBlock.CrossTxTokenPrivacyData,
						OutputCoin:       toShardCrossShardBlock.CrossOutputCoin,
						BlockHash:        *toShardCrossShardBlock.Hash(),
						BlockHeight:      toShardCrossShardBlock.Header.Height,
					}
					targetHash := crossTransaction.Hash()
					hash := compareCrossTransaction.Hash()
					if !hash.IsEqual(&targetHash) {
						return NewBlockChainError(CrossTransactionHashError, fmt.Errorf("Cross Output Coin From New Block %+v not compatible with cross shard block in pool %+v", targetHash, hash))
					}
					if true {
						toShardCrossShardBlocks = toShardCrossShardBlocks[index:]
						isValids++
						break
					}
				}
			}
		}
		if len(crossTransactions) != isValids {
			return NewBlockChainError(CrossShardBlockError, fmt.Errorf("Can't not verify all cross shard block from shard %+v", fromShard))
		}
	}
	shardVerifyPreprocesingForPreSignTimer.UpdateSince(startTimeVerifyPreProcessingShardBlockForSigning)
	return nil
}

// verifyBestStateWithShardBlock will verify the validation of a block with some best state in cache or current best state
// Get beacon state of this block
// For example, new blockHeight is 91 then beacon state of this block must have height 90
// OR new block has previous has is beacon best block hash
//	- New Shard Block has parent (previous) hash is current shard state best block hash (compatible with current beststate)
//	- New Shard Block Height must be compatible with best shard state
//	- New Shard Block has beacon must higher or equal to beacon height of shard best state
func (shardBestState *ShardBestState) verifyBestStateWithShardBlock(blockchain *BlockChain, shardBlock *ShardBlock, isVerifySig bool, shardID byte) error {
	startTimeVerifyBestStateWithShardBlock := time.Now()
	Logger.log.Debugf("SHARD %+v | Begin VerifyBestStateWithShardBlock Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	//verify producer via index
	if err := blockchain.config.ConsensusEngine.ValidateProducerPosition(shardBlock, shardBestState.ShardProposerIdx, shardBestState.ShardCommittee); err != nil {
		return err
	}
	// check with current final best state
	// shardBlock can only be insert if it match the current best state
	if !shardBestState.BestBlockHash.IsEqual(&shardBlock.Header.PreviousBlockHash) {
		return NewBlockChainError(ShardBestStateNotCompatibleError, fmt.Errorf("Current Best Block Hash %+v, New Shard Block %+v, Previous Block Hash of New Block %+v", shardBestState.BestBlockHash, shardBlock.Header.Height, shardBlock.Header.PreviousBlockHash))
	}
	if shardBestState.ShardHeight+1 != shardBlock.Header.Height {
		return NewBlockChainError(WrongBlockHeightError, fmt.Errorf("Shard Block height of new Shard Block should be %+v, but get %+v", shardBestState.ShardHeight+1, shardBlock.Header.Height))
	}
	if shardBlock.Header.BeaconHeight < shardBestState.BeaconHeight {
		return NewBlockChainError(ShardBestStateBeaconHeightNotCompatibleError, fmt.Errorf("Shard Block contain invalid beacon height, current beacon height %+v but get %+v ", shardBestState.BeaconHeight, shardBlock.Header.BeaconHeight))
	}
	shardVerifyWithBestStateTimer.UpdateSince(startTimeVerifyBestStateWithShardBlock)
	Logger.log.Debugf("SHARD %+v | Finish VerifyBestStateWithShardBlock Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	return nil
}

// updateShardBestState beststate with new shard block:
//	- New Previous Shard BlockHash
//	- New BestShardBlockHash
//	- New BestBeaconHash
//	- New Best Shard Block
//	- New Best Shard Height
//	- New Beacon Height
//	- ShardProposerIdx of new shard block
//	- Execute stake instruction, store staking transaction (if exist)
//	- Execute assign instruction, add new pending validator (if exist)
//	- Execute swap instruction, swap pending validator and committee (if exist)
func (oldBestState *ShardBestState) updateShardBestState(blockchain *BlockChain, shardBlock *ShardBlock, beaconBlocks []*BeaconBlock, committeeChange *committeeChange) (*ShardBestState, error) {
	startTimeUpdateShardBestState := time.Now()
	Logger.log.Debugf("SHARD %+v | Begin update Beststate with new Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	shardBestState := NewShardBestState()
	if err := shardBestState.cloneShardBestStateFrom(oldBestState); err != nil {
		return nil, err
	}
	var (
		err     error
		shardID = shardBlock.Header.ShardID
	)
	shardBestState.BestBlockHash = *shardBlock.Hash()
	shardBestState.BestBeaconHash = shardBlock.Header.BeaconHash
	shardBestState.BestBlock = shardBlock
	shardBestState.BestBlockHash = *shardBlock.Hash()
	shardBestState.ShardHeight = shardBlock.Header.Height
	shardBestState.Epoch = shardBlock.Header.Epoch
	shardBestState.BeaconHeight = shardBlock.Header.BeaconHeight
	shardBestState.TotalTxns += uint64(len(shardBlock.Body.Transactions))
	shardBestState.NumTxns = uint64(len(shardBlock.Body.Transactions))
	if shardBlock.Header.Height == 1 {
		shardBestState.ShardProposerIdx = 0
	} else {
		for i, v := range oldBestState.ShardCommittee {
			b58Str, _ := v.ToBase58()
			if b58Str == shardBlock.Header.Producer {
				shardBestState.ShardProposerIdx = i
				break
			}
		}
	}

	//shardBestState.processBeaconBlocks(shardBlock, beaconBlocks)
	shardPendingValidator, newShardPendingValidator, stakingTx := blockchain.processInstructionFromBeacon(oldBestState, beaconBlocks, shardBlock.Header.ShardID, committeeChange)
	shardBestState.ShardPendingValidator, err = incognitokey.CommitteeBase58KeyListToStruct(shardPendingValidator)
	if err != nil {
		return nil, err
	}
	committeeChange.shardSubstituteAdded[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(newShardPendingValidator)
	if err != nil {
		return nil, err
	}
	for stakePublicKey, txHash := range stakingTx {
		shardBestState.StakingTx[stakePublicKey] = txHash
	}
	err = shardBestState.processShardBlockInstruction(blockchain, shardBlock, committeeChange)
	if err != nil {
		return nil, err
	}
	//updateShardBestState best cross shard
	for shardID, crossShardBlock := range shardBlock.Body.CrossTransactions {
		shardBestState.BestCrossShard[shardID] = crossShardBlock[len(crossShardBlock)-1].BlockHeight
	}
	//======BEGIN For testing and benchmark
	temp := 0
	for _, tx := range shardBlock.Body.Transactions {
		//detect transaction that's not salary
		if !tx.IsSalaryTx() {
			temp++
		}
	}
	shardBestState.TotalTxnsExcludeSalary += uint64(temp)
	//======END
	shardUpdateBestStateTimer.UpdateSince(startTimeUpdateShardBestState)
	Logger.log.Debugf("SHARD %+v | Finish update Beststate with new Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	return shardBestState, nil
}

func (shardBestState *ShardBestState) initShardBestState(blockchain *BlockChain, db incdb.Database, genesisShardBlock *ShardBlock, genesisBeaconBlock *BeaconBlock) error {
	shardBestState.BestBeaconHash = *ChainTestParam.GenesisBeaconBlock.Hash()
	shardBestState.BestBlock = genesisShardBlock
	shardBestState.BestBlockHash = *genesisShardBlock.Hash()
	shardBestState.ShardHeight = genesisShardBlock.Header.Height
	shardBestState.Epoch = genesisShardBlock.Header.Epoch
	shardBestState.BeaconHeight = genesisShardBlock.Header.BeaconHeight
	shardBestState.TotalTxns += uint64(len(genesisShardBlock.Body.Transactions))
	shardBestState.NumTxns = uint64(len(genesisShardBlock.Body.Transactions))
	shardBestState.ShardProposerIdx = 0
	//shardBestState.processBeaconBlocks(genesisShardBlock, []*BeaconBlock{genesisBeaconBlock})
	shardPendingValidator, _, stakingTx := blockchain.processInstructionFromBeacon(nil, []*BeaconBlock{genesisBeaconBlock}, genesisShardBlock.Header.ShardID, newCommitteeChange())

	shardPendingValidatorStr, err := incognitokey.CommitteeBase58KeyListToStruct(shardPendingValidator)
	if err != nil {
		return err
	}
	shardBestState.ShardPendingValidator = append(shardBestState.ShardPendingValidator, shardPendingValidatorStr...)
	for stakePublicKey, txHash := range stakingTx {
		shardBestState.StakingTx[stakePublicKey] = txHash
	}
	err = shardBestState.processShardBlockInstruction(blockchain, genesisShardBlock, newCommitteeChange())
	if err != nil {
		return err
	}
	shardBestState.ConsensusAlgorithm = common.BlsConsensus
	shardBestState.NumOfBlocksByProducers = make(map[string]uint64)
	//statedb===========================START
	dbAccessWarper := statedb.NewDatabaseAccessWarper(db)
	shardBestState.consensusStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	shardBestState.transactionStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	shardBestState.featureStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	shardBestState.rewardStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	shardBestState.slashStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	shardBestState.ConsensusStateDBRootHash = common.EmptyRoot
	shardBestState.SlashStateDBRootHash = common.EmptyRoot
	shardBestState.RewardStateDBRootHash = common.EmptyRoot
	shardBestState.FeatureStateDBRootHash = common.EmptyRoot
	shardBestState.TransactionStateDBRootHash = common.EmptyRoot
	//statedb===========================END
	return nil
}

func (shardBestState *ShardBestState) processShardBlockInstruction(blockchain *BlockChain, shardBlock *ShardBlock, committeeChange *committeeChange) error {
	var err error
	shardID := shardBestState.ShardID
	shardPendingValidator, err := incognitokey.CommitteeKeyListToString(shardBestState.ShardPendingValidator)
	if err != nil {
		return err
	}
	shardCommittee, err := incognitokey.CommitteeKeyListToString(shardBestState.ShardCommittee)
	if err != nil {
		return err
	}
	// TODO: 0xmerman
	fixedProducerShardValidators := shardCommittee[:NumberOfFixedBlockValidators]
	shardCommittee = shardCommittee[NumberOfFixedBlockValidators:]

	shardSwappedCommittees := []string{}
	shardNewCommittees := []string{}
	if len(shardBlock.Body.Instructions) != 0 {
		Logger.log.Debugf("Shard Process/updateShardBestState: Shard Instruction %+v", shardBlock.Body.Instructions)
	}
	producersBlackList, err := blockchain.getUpdatedProducersBlackList(blockchain.GetBeaconBestState().slashStateDB, false, int(shardID), shardCommittee, shardBlock.Header.BeaconHeight)
	if err != nil {
		return err
	}
	// Swap committee
	for _, l := range shardBlock.Body.Instructions {
		if l[0] == SwapAction {
			// #1 remaining pendingValidators, #2 new currentValidators #3 swapped out validator, #4 incoming validator
			maxShardCommitteeSize := shardBestState.MaxShardCommitteeSize - NumberOfFixedBlockValidators
			var minShardCommitteeSize int
			if shardBestState.MinShardCommitteeSize-NumberOfFixedBlockValidators < 0 {
				minShardCommitteeSize = 0
			} else {
				minShardCommitteeSize = shardBestState.MinShardCommitteeSize - NumberOfFixedBlockValidators
			}
			shardPendingValidator, shardCommittee, shardSwappedCommittees, shardNewCommittees, err = SwapValidator(shardPendingValidator, shardCommittee, maxShardCommitteeSize, minShardCommitteeSize, blockchain.config.ChainParams.Offset, producersBlackList, blockchain.config.ChainParams.SwapOffset)
			if err != nil {
				Logger.log.Errorf("SHARD %+v | Blockchain Error %+v", err)
				return NewBlockChainError(SwapValidatorError, err)
			}
			swapedCommittees := []string{}
			if len(l[2]) != 0 && l[2] != "" {
				swapedCommittees = strings.Split(l[2], ",")
			}
			for _, v := range swapedCommittees {
				if txId, ok := shardBestState.StakingTx[v]; ok {
					if checkReturnStakingTxExistence(txId, shardBlock) {
						delete(shardBestState.StakingTx, v)
					}
				}
			}
			if !reflect.DeepEqual(swapedCommittees, shardSwappedCommittees) {
				return NewBlockChainError(SwapValidatorError, fmt.Errorf("Expect swapped committees to be %+v but get %+v", swapedCommittees, shardSwappedCommittees))
			}
			newCommittees := []string{}
			if len(l[1]) > 0 {
				newCommittees = strings.Split(l[1], ",")
			}
			if !reflect.DeepEqual(newCommittees, shardNewCommittees) {
				return NewBlockChainError(SwapValidatorError, fmt.Errorf("Expect new committees to be %+v but get %+v", newCommittees, shardNewCommittees))
			}
			shardNewCommitteesStruct, err := incognitokey.CommitteeBase58KeyListToStruct(shardNewCommittees)
			if err != nil {
				return err
			}
			shardSwappedCommitteesStruct, err := incognitokey.CommitteeBase58KeyListToStruct(shardSwappedCommittees)
			if err != nil {
				return err
			}
			beforeFilterShardSubstituteAdded := committeeChange.shardSubstituteAdded[shardID]
			filteredShardSubstituteAdded := []incognitokey.CommitteePublicKey{}
			filteredShardSubstituteRemoved := []incognitokey.CommitteePublicKey{}
			for _, currentShardSubstituteAdded := range beforeFilterShardSubstituteAdded {
				flag := false
				for _, newShardCommitteeAdded := range shardNewCommitteesStruct {
					if currentShardSubstituteAdded.IsEqual(newShardCommitteeAdded) {
						flag = true
						break
					}
				}
				if !flag {
					filteredShardSubstituteAdded = append(filteredShardSubstituteAdded, currentShardSubstituteAdded)
				}
			}
			for _, newShardCommitteeAdded := range shardNewCommitteesStruct {
				flag := false
				for _, currentShardSubstituteAdded := range beforeFilterShardSubstituteAdded {
					if currentShardSubstituteAdded.IsEqual(newShardCommitteeAdded) {
						flag = true
						break
					}
				}
				if !flag {
					filteredShardSubstituteRemoved = append(filteredShardSubstituteRemoved, newShardCommitteeAdded)
				}
			}
			committeeChange.shardCommitteeAdded[shardID] = shardNewCommitteesStruct
			committeeChange.shardCommitteeRemoved[shardID] = shardSwappedCommitteesStruct
			committeeChange.shardSubstituteRemoved[shardID] = filteredShardSubstituteRemoved
			committeeChange.shardSubstituteAdded[shardID] = filteredShardSubstituteAdded
			Logger.log.Infof("SHARD %+v | Swap: Out committee %+v", shardID, shardSwappedCommittees)
			Logger.log.Infof("SHARD %+v | Swap: In committee %+v", shardID, shardNewCommittees)
		}
	}
	shardBestState.ShardPendingValidator, err = incognitokey.CommitteeBase58KeyListToStruct(shardPendingValidator)
	if err != nil {
		return err
	}
	//TODO: merman
	shardBestState.ShardCommittee, err = incognitokey.CommitteeBase58KeyListToStruct(append(fixedProducerShardValidators, shardCommittee...))
	if err != nil {
		return err
	}
	return nil
}

// verifyPostProcessingShardBlock
//	- commitee root
//	- pending validator root
func (shardBestState *ShardBestState) verifyPostProcessingShardBlock(shardBlock *ShardBlock, shardID byte) error {
	startTimeVerifyPostProcessingShardBlock := time.Now()
	Logger.log.Debugf("SHARD %+v | Begin VerifyPostProcessing Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())

	shardCommitteeStr, err := incognitokey.CommitteeKeyListToString(shardBestState.ShardCommittee)
	if err != nil {
		return err
	}
	if hash, ok := verifyHashFromStringArray(shardCommitteeStr, shardBlock.Header.CommitteeRoot); !ok {
		return NewBlockChainError(ShardCommitteeRootHashError, fmt.Errorf("Expect shard committee root hash to be %+v but get %+v", shardBlock.Header.CommitteeRoot, hash))
	}

	shardPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(shardBestState.ShardPendingValidator)
	if err != nil {
		return err
	}
	if hash, isOk := verifyHashFromStringArray(shardPendingValidatorStr, shardBlock.Header.PendingValidatorRoot); !isOk {
		return NewBlockChainError(ShardPendingValidatorRootHashError, fmt.Errorf("Expect shard pending validator root hash to be %+v but get %+v", shardBlock.Header.PendingValidatorRoot, hash))
	}
	if hash, isOk := verifyHashFromMapStringString(shardBestState.StakingTx, shardBlock.Header.StakingTxRoot); !isOk {
		return NewBlockChainError(ShardPendingValidatorRootHashError, fmt.Errorf("Expect shard staking root hash to be %+v but get %+v", shardBlock.Header.StakingTxRoot, hash))
	}
	shardVerifyPostProcessingTimer.UpdateSince(startTimeVerifyPostProcessingShardBlock)
	Logger.log.Debugf("SHARD %+v | Finish VerifyPostProcessing Block with height %+v at hash %+v", shardBlock.Header.ShardID, shardBlock.Header.Height, shardBlock.Hash())
	return nil
}

// Verify Transaction with these condition:
//	1. Validate tx version
//	2. Validate fee with tx size
//	3. Validate type of tx
//	4. Validate with other txs in block:
// 	- Normal Transaction:
// 	- Custom Tx:
//	4.1 Validate Init Custom Token
//	5. Validate sanity data of tx
//	6. Validate data in tx: privacy proof, metadata,...
//	7. Validate tx with blockchain: douple spend, ...
//	8. Check tx existed in block
//	9. Not accept a salary tx
//	10. Check duplicate staker public key in block
//	11. Check duplicate Init Custom Token in block
func (blockchain *BlockChain) verifyTransactionFromNewBlock(shardID byte, txs []metadata.Transaction, beaconHeight int64, curView *ShardBestState) error {
	if len(txs) == 0 {
		return nil
	}
	isEmpty := blockchain.config.TempTxPool.EmptyPool()
	if !isEmpty {
		panic("TempTxPool Is not Empty")
	}
	defer blockchain.config.TempTxPool.EmptyPool()
	listTxs := []metadata.Transaction{}
	for _, tx := range txs {
		if !tx.IsSalaryTx() {
			listTxs = append(listTxs, tx)
		}
	}
	_, err := blockchain.config.TempTxPool.MaybeAcceptBatchTransactionForBlockProducing(shardID, listTxs, beaconHeight, curView)
	if err != nil {
		Logger.log.Errorf("Batching verify transactions from new block err: %+v\n Trying verify one by one", err)
		for index, tx := range listTxs {
			if blockchain.config.TempTxPool.HaveTransaction(tx.Hash()) {
				continue
			}
			_, err1 := blockchain.config.TempTxPool.MaybeAcceptTransactionForBlockProducing(tx, beaconHeight, curView)
			if err1 != nil {
				Logger.log.Errorf("One by one verify txs at index %d error: %+v", index, err1)
				return NewBlockChainError(TransactionFromNewBlockError, fmt.Errorf("Transaction %+v, index %+v get %+v ", *tx.Hash(), index, err1))
			}
		}
	}
	return nil
}

// processStoreShardBlock Store All information after Insert
//	- Shard Block
//	- Shard Best State
//	- Transaction => UTXO, serial number, snd, commitment
//	- Cross Output Coin => UTXO, snd, commmitment
//	- Store transaction metadata:
//		+ Withdraw Metadata
//	- Store incoming cross shard block
//	- Store Burning Confirmation
//	- Update Mempool fee estimator
func (blockchain *BlockChain) processStoreShardBlock(newShardState *ShardBestState, shardBlock *ShardBlock, committeeChange *committeeChange, beaconBlock *BeaconBlock) error {
	startTimeProcessStoreShardBlock := time.Now()
	shardID := shardBlock.Header.ShardID
	blockHeight := shardBlock.Header.Height
	blockHash := shardBlock.Header.Hash()

	Logger.log.Infof("SHARD %+v | Process store block height %+v at hash %+v", shardBlock.Header.ShardID, blockHeight, *shardBlock.Hash())
	if len(shardBlock.Body.CrossTransactions) != 0 {
		Logger.log.Critical("processStoreShardBlock/CrossTransactions	", shardBlock.Body.CrossTransactions)
	}
	if err := blockchain.CreateAndSaveTxViewPointFromBlock(shardBlock, newShardState.transactionStateDB); err != nil {
		return NewBlockChainError(FetchAndStoreTransactionError, err)
	}

	for index, tx := range shardBlock.Body.Transactions {
		if err := rawdbv2.StoreTransactionIndex(blockchain.GetShardChainDatabase(shardID), *tx.Hash(), shardBlock.Header.Hash(), index); err != nil {
			return NewBlockChainError(FetchAndStoreTransactionError, err)
		}
		// Process Transaction Metadata
		metaType := tx.GetMetadataType()
		if metaType == metadata.WithDrawRewardResponseMeta {
			_, publicKey, amountRes, coinID := tx.GetTransferData()
			err := statedb.RemoveCommitteeReward(newShardState.rewardStateDB, publicKey, amountRes, *coinID)
			if err != nil {
				return NewBlockChainError(RemoveCommitteeRewardError, err)
			}
		}
		Logger.log.Debugf("Transaction in block with hash", blockHash, "and index", index)
	}
	// Store Incomming Cross Shard
	if err := blockchain.CreateAndSaveCrossTransactionViewPointFromBlock(shardBlock, newShardState.transactionStateDB); err != nil {
		return NewBlockChainError(FetchAndStoreCrossTransactionError, err)
	}
	// Save result of BurningConfirm instruction to get proof later
	err := blockchain.storeBurningConfirm(newShardState.featureStateDB, shardBlock)
	if err != nil {
		return NewBlockChainError(StoreBurningConfirmError, err)
	}
	// Update bridge issuance request status
	err = blockchain.updateBridgeIssuanceStatus(newShardState.featureStateDB, shardBlock)
	if err != nil {
		return NewBlockChainError(UpdateBridgeIssuanceStatusError, err)
	}
	// call FeeEstimator for processing
	if feeEstimator, ok := blockchain.config.FeeEstimator[shardBlock.Header.ShardID]; ok {
		err := feeEstimator.RegisterBlock(shardBlock)
		if err != nil {
			Logger.log.Debug(NewBlockChainError(RegisterEstimatorFeeError, err))
		}
	}
	if len(committeeChange.shardCommitteeAdded[shardID]) > 0 || len(committeeChange.shardSubstituteAdded[shardID]) > 0 {
		beaconConsensusStateRootHash, err := blockchain.GetBeaconConsensusRootHash(blockchain.GetBeaconChainDatabase(), beaconBlock.Header.Height)
		if err != nil {
			return NewBlockChainError(StoreShardBlockError, fmt.Errorf("can't get ConsensusStateRootHash of height %+v ,error %+v", beaconBlock.Header.Height, err))
		}
		beaconConsensusStateDB, err := statedb.NewWithPrefixTrie(beaconConsensusStateRootHash, statedb.NewDatabaseAccessWarper(blockchain.GetBeaconChainDatabase()))
		if err != nil {
			return NewBlockChainError(StoreShardBlockError, err)
		}
		rewardReceiver, autoStaking := statedb.GetRewardReceiverAndAutoStaking(beaconConsensusStateDB, blockchain.GetShardIDs())
		//statedb===========================START
		err = statedb.StoreOneShardCommittee(newShardState.consensusStateDB, shardID, committeeChange.shardCommitteeAdded[shardID], rewardReceiver, autoStaking)
		if err != nil {
			return NewBlockChainError(StoreShardBlockError, err)
		}
		err = statedb.StoreOneShardSubstitutesValidator(newShardState.consensusStateDB, shardID, committeeChange.shardSubstituteAdded[shardID], rewardReceiver, autoStaking)
		if err != nil {
			return NewBlockChainError(StoreShardBlockError, err)
		}
	}
	err = statedb.DeleteOneShardCommittee(newShardState.consensusStateDB, shardID, committeeChange.shardCommitteeAdded[shardID])
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = statedb.DeleteOneShardSubstitutesValidator(newShardState.consensusStateDB, shardID, committeeChange.shardSubstituteAdded[shardID])
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	// consensus root hash
	consensusRootHash, err := newShardState.consensusStateDB.Commit(true)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = newShardState.consensusStateDB.Database().TrieDB().Commit(consensusRootHash, false)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	newShardState.ConsensusStateDBRootHash = consensusRootHash
	// transaction root hash
	transactionRootHash, err := newShardState.transactionStateDB.Commit(true)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = newShardState.transactionStateDB.Database().TrieDB().Commit(transactionRootHash, false)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	newShardState.TransactionStateDBRootHash = transactionRootHash
	// feature root hash
	featureRootHash, err := newShardState.featureStateDB.Commit(true)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = newShardState.featureStateDB.Database().TrieDB().Commit(featureRootHash, false)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	newShardState.FeatureStateDBRootHash = featureRootHash
	// reward root hash
	rewardRootHash, err := newShardState.rewardStateDB.Commit(true)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = newShardState.rewardStateDB.Database().TrieDB().Commit(rewardRootHash, false)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	newShardState.RewardStateDBRootHash = rewardRootHash
	// slash root hash
	slashRootHash, err := newShardState.slashStateDB.Commit(true)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	err = newShardState.slashStateDB.Database().TrieDB().Commit(slashRootHash, false)
	if err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	newShardState.consensusStateDB.ClearObjects()
	newShardState.transactionStateDB.ClearObjects()
	newShardState.featureStateDB.ClearObjects()
	newShardState.rewardStateDB.ClearObjects()
	newShardState.slashStateDB.ClearObjects()

	batchData := blockchain.GetShardChainDatabase(shardID).NewBatch()
	if err := rawdbv2.StoreShardConsensusRootHash(batchData, shardID, blockHeight, consensusRootHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	if err := rawdbv2.StoreShardTransactionRootHash(batchData, shardID, blockHeight, transactionRootHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	if err := rawdbv2.StoreShardFeatureRootHash(batchData, shardID, blockHeight, featureRootHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	if err := rawdbv2.StoreShardCommitteeRewardRootHash(batchData, shardID, blockHeight, rewardRootHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	if err := rawdbv2.StoreShardSlashRootHash(batchData, shardID, blockHeight, slashRootHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	//statedb===========================END
	if err := rawdbv2.StoreShardBlock(batchData, shardID, blockHeight, blockHash, shardBlock); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	if err := rawdbv2.StoreShardBlockIndex(batchData, shardID, blockHeight, blockHash); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}

	Logger.log.Infof("SHARD %+v | 🔎 Remove outdate branch")
	finalView := blockchain.ShardChain[shardBlock.Header.ShardID].GetFinalView()
	blockchain.ShardChain[shardBlock.Header.ShardID].multiView.AddView(newShardState)
	newFinalView := blockchain.ShardChain[shardBlock.Header.ShardID].GetFinalView()
	if finalView != nil && newFinalView.GetHash().String() != finalView.GetHash().String() {
		startView := newFinalView
		for {
			blks, _ := blockchain.GetShardBlockHashByHeight(startView.GetHeight(), shardID)
			for _, hash := range blks {
				if hash.String() != startView.GetHash().String() {
					err := rawdbv2.DeleteShardBlock(blockchain.GetShardChainDatabase(shardID), shardID, startView.GetHeight(), hash)
					if err != nil {
						//must not have error
						panic(err)
					}
				}
			}
			startView = blockchain.ShardChain[shardBlock.Header.ShardID].GetViewByHash(*startView.GetPreviousHash())
			if startView == nil || startView.GetHeight() == finalView.GetHeight() {
				break
			}
		}
	}

	err = blockchain.BackupShardViews(batchData, shardBlock.Header.ShardID)
	if err != nil {
		panic("Backup shard view error")
	}

	if err := batchData.Write(); err != nil {
		return NewBlockChainError(StoreShardBlockError, err)
	}
	shardStoreBlockTimer.UpdateSince(startTimeProcessStoreShardBlock)
	Logger.log.Infof("SHARD %+v | 🔎 %d transactions in block height %+v \n", shardBlock.Header.ShardID, len(shardBlock.Body.Transactions), blockHeight)
	return nil
}

// removeOldDataAfterProcessingShardBlock remove outdate data from pool and beststate
//	- Remove Staking TX in Shard BestState from instruction
//	- Set Shard State for removing old Shard Block in Pool
//	- Remove Old Cross Shard Block
//	- Remove Init Tokens ID in Mempool
//	- Remove Candiates in Mempool
//	- Remove Transaction in Mempool and Block Generator
func (blockchain *BlockChain) removeOldDataAfterProcessingShardBlock(shardBlock *ShardBlock, shardID byte) {
	go func() {
		//Remove Candidate In pool
		candidates := []string{}
		for _, tx := range shardBlock.Body.Transactions {
			if blockchain.config.IsBlockGenStarted {
				blockchain.config.CRemovedTxs <- tx
			}
			if tx.GetMetadata() != nil {
				if tx.GetMetadata().GetType() == metadata.ShardStakingMeta || tx.GetMetadata().GetType() == metadata.BeaconStakingMeta {
					stakingMetadata, ok := tx.GetMetadata().(*metadata.StakingMetadata)
					if !ok {
						continue
					}
					candidates = append(candidates, stakingMetadata.CommitteePublicKey)
				}
			}
		}
		go blockchain.config.TxPool.RemoveCandidateList(candidates)
		//Remove tx out of pool
		go blockchain.config.TxPool.RemoveTx(shardBlock.Body.Transactions, true)
	}()
}
