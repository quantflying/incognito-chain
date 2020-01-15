package block

import (
	"errors"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/transaction"

	"sort"
	"strconv"
	"strings"
)

type ShardCoreApp struct {
	Logger        common.Logger
	CreateState   *CreateShardBlockState
	ValidateState *ValidateShardBlockState
}

//==============================Create Block Logic===========================
func (s *ShardCoreApp) preCreateBlock() error {
	state := s.CreateState
	bc := state.bc
	beaconHeight, err := state.bc.GetCurrentBeaconHeight()
	if err != nil {
		return err
	}

	curView := state.curView

	if beaconHeight < curView.GetBlock().GetBeaconHeight() {
		//this case cannot happen, as view only add when we verify that beacon is confirm
		panic("something wrong")
	}
	// limit number of beacon block confirmed in a shard block
	if beaconHeight > curView.GetBlock().GetBeaconHeight() && beaconHeight-curView.GetBlock().GetBeaconHeight() > blockchain.MAX_BEACON_BLOCK {
		beaconHeight = curView.GetBlock().GetBeaconHeight() + blockchain.MAX_BEACON_BLOCK
	}

	// we only confirm shard blocks within an epoch
	epoch := beaconHeight / bc.GetChainParams().Epoch

	if epoch > curView.GetEpoch() { // if current epoch > curView 1 epoch, we only confirm beacon block within an epoch
		beaconHeight = curView.GetEpoch() * bc.GetChainParams().Epoch
		epoch = curView.GetEpoch() + 1
	}

	state.newBlockEpoch = epoch
	state.newConfirmBeaconHeight = beaconHeight
	toShard := state.curView.ShardID
	state.crossShardBlocks = state.bc.GetAllValidCrossShardBlockFromPool(toShard)
	//TODO: get beacon blocks

	return nil
}

func (s *ShardCoreApp) buildTxFromCrossShard() error {
	state := s.CreateState
	toShard := state.curView.ShardID
	crossTransactions := make(map[byte][]blockchain.CrossTransaction)
	// Get Cross Shard Block
	for fromShard, crossShardBlock := range state.crossShardBlocks {
		sort.SliceStable(crossShardBlock[:], func(i, j int) bool {
			return crossShardBlock[i].Header.Height < crossShardBlock[j].Header.Height
		})
		indexs := []int{}
		startHeight := state.bc.GetLatestCrossShard(fromShard, toShard)
		for index, crossShardBlock := range crossShardBlock {
			if crossShardBlock.Header.Height <= startHeight {
				break
			}
			nextHeight := state.bc.GetNextCrossShard(fromShard, toShard, startHeight)

			if nextHeight > crossShardBlock.Header.Height {
				continue
			}

			if nextHeight < crossShardBlock.Header.Height {
				break
			}

			startHeight = nextHeight

			err := state.bc.ValidateCrossShardBlock(crossShardBlock)
			if err != nil {
				break
			}
			indexs = append(indexs, index)
		}

		for _, index := range indexs {
			blk := crossShardBlock[index]
			crossTransaction := blockchain.CrossTransaction{
				OutputCoin:       blk.CrossOutputCoin,
				TokenPrivacyData: blk.CrossTxTokenPrivacyData,
				BlockHash:        *blk.Hash(),
				BlockHeight:      blk.Header.Height,
			}
			crossTransactions[blk.Header.ShardID] = append(crossTransactions[blk.Header.ShardID], crossTransaction)
		}
	}

	for _, crossTransaction := range crossTransactions {
		sort.SliceStable(crossTransaction[:], func(i, j int) bool {
			return crossTransaction[i].BlockHeight < crossTransaction[j].BlockHeight
		})
	}
	state.crossShardTx = crossTransactions
	return nil
}

func (s *ShardCoreApp) buildTxFromMemPool() error {
	state := s.CreateState
	txsToAdd, txToRemove, _ := state.bc.GetPendingTransaction(state.curView.ShardID)
	if len(txsToAdd) == 0 {
		s.Logger.Info("Creating empty block...")
	}
	state.txToRemoveFromPool = txToRemove
	state.txsToAddFromPool = txsToAdd
	return nil
}

func (s *ShardCoreApp) buildResponseTxFromTxWithMetadata() error {
	state := s.CreateState
	blkProducerPrivateKey := createTempKeyset()
	txRequestTable := map[string]metadata.Transaction{}
	txsRes := []metadata.Transaction{}
	//process withdraw reward
	for _, tx := range state.txsToAddFromPool {
		if tx.GetMetadataType() == metadata.WithDrawRewardRequestMeta {
			requestMeta := tx.GetMetadata().(*metadata.WithDrawRewardRequest)
			requester := base58.Base58Check{}.Encode(requestMeta.PaymentAddress.Pk, common.Base58Version)
			txRequestTable[requester] = tx
		}
	}
	for _, value := range txRequestTable {
		txRes, err := s.buildWithDrawTransactionResponse(&value, &blkProducerPrivateKey)
		if err != nil {
			return err
		} else {
			s.Logger.Infof("[Reward] - BuildWithDrawTransactionResponse for tx %+v, ok: %+v\n", value, txRes)
		}
		txsRes = append(txsRes, txRes)
	}
	state.txsFromMetadataTx = txsRes
	return nil
}

func (s *ShardCoreApp) processBeaconInstruction() error {
	state := s.CreateState
	shardID := state.curView.ShardID
	producerPrivateKey := createTempKeyset()
	responsedTxs := []metadata.Transaction{}
	shardPendingValidator, err := incognitokey.CommitteeKeyListToString(state.bc.GetShardPendingCommittee(state.curView.ShardID))
	if err != nil {
		panic(err)
	}
	assignInstructions := [][]string{}
	stakingTx := make(map[string]string)
	for _, beaconBlock := range state.beaconBlocks {
		for _, l := range beaconBlock.GetInstructions() {

			if l[0] == blockchain.SwapAction {
				autoStaking, err := state.bc.FetchAutoStakingByHeight(beaconBlock.GetHeight())
				if err != nil {
					break
				}
				for _, outPublicKeys := range strings.Split(l[2], ",") {
					// If out public key has auto staking then ignore this public key
					if _, ok := autoStaking[outPublicKeys]; ok {
						continue
					}
					tx, err := s.buildReturnStakingAmountTx(outPublicKeys, &producerPrivateKey)
					if err != nil {
						s.Logger.Error(err)
						continue
					}
					responsedTxs = append(responsedTxs, tx)
				}
			}

			// Process Assign Instruction
			if l[0] == blockchain.AssignAction && l[2] == "shard" {
				if strings.Compare(l[3], strconv.Itoa(int(shardID))) == 0 {
					shardPendingValidator = append(shardPendingValidator, strings.Split(l[1], ",")...)
					assignInstructions = append(assignInstructions, l)
				}
			}
			// Get Staking Tx
			// assume that stake instruction already been validated by beacon committee
			if l[0] == blockchain.StakeAction && l[2] == "beacon" {
				beacon := strings.Split(l[1], ",")
				newBeaconCandidates := []string{}
				newBeaconCandidates = append(newBeaconCandidates, beacon...)
				if len(l) == 6 {
					for i, v := range strings.Split(l[3], ",") {
						txHash, err := common.Hash{}.NewHashFromStr(v)
						if err != nil {
							continue
						}
						txShardID, _, _, _, err := state.bc.GetTransactionByHash(*txHash)
						if err != nil {
							continue
						}
						if txShardID != shardID {
							continue
						}
						// if transaction belong to this shard then add to shard beststate
						stakingTx[newBeaconCandidates[i]] = v
					}
				}
			}
			if l[0] == blockchain.StakeAction && l[2] == "shard" {
				shard := strings.Split(l[1], ",")
				newShardCandidates := []string{}
				newShardCandidates = append(newShardCandidates, shard...)
				if len(l) == 6 {
					for i, v := range strings.Split(l[3], ",") {
						txHash, err := common.Hash{}.NewHashFromStr(v)
						if err != nil {
							continue
						}
						txShardID, _, _, _, err := state.bc.GetTransactionByHash(*txHash)
						if err != nil {
							continue
						}
						if txShardID != shardID {
							continue
						}
						// if transaction belong to this shard then add to shard beststate
						stakingTx[newShardCandidates[i]] = v
					}
				}
			}
		}
	}
	state.newShardPendingValidator = shardPendingValidator
	state.stakingTx = stakingTx
	state.txsFromBeaconInstruction = responsedTxs
	return nil
}

func (s *ShardCoreApp) generateInstruction() error {
	state := s.CreateState
	beaconHeight := state.newConfirmBeaconHeight
	shardCommittee, _ := incognitokey.CommitteeKeyListToString(state.curView.ShardCommittee)
	shardID := state.curView.ShardID
	var (
		instructions          = [][]string{}
		swapInstruction       = []string{}
		shardPendingValidator = state.newShardPendingValidator
		err                   error
	)
	if beaconHeight%state.bc.GetChainParams().Epoch == 0 && state.curView.Block.Header.BeaconHeight == beaconHeight {
		maxShardCommitteeSize := state.bc.GetChainParams().MaxShardCommitteeSize
		minShardCommitteeSize := state.bc.GetChainParams().MinShardCommitteeSize

		s.Logger.Info("ShardPendingValidator", state.newShardPendingValidator)
		s.Logger.Info("ShardCommittee", shardCommittee)
		s.Logger.Info("MaxShardCommitteeSize", maxShardCommitteeSize)
		s.Logger.Info("ShardID", shardID)

		swapInstruction, shardPendingValidator, shardCommittee, err = blockchain.CreateSwapAction(shardPendingValidator, shardCommittee, maxShardCommitteeSize, minShardCommitteeSize, shardID, map[string]uint8{}, map[string]uint8{}, state.bc.GetChainParams().Offset, state.bc.GetChainParams().SwapOffset)
		if err != nil {
			s.Logger.Error(err)
			return err
		}
	}
	if len(swapInstruction) > 0 {
		instructions = append(instructions, swapInstruction)
	}
	state.instruction = instructions
	return nil
}

func (s *ShardCoreApp) buildHeader() error {
	state := s.CreateState
	curView := state.curView
	// create new view
	newViewInterface, err := curView.CreateNewViewFromBlock(s.CreateState.newBlock)
	if err != nil {
		return err
	}
	newView := newViewInterface.(*ShardView)

	shardID := state.curView.ShardID
	totalTxsFee := make(map[common.Hash]uint64)
	for _, tx := range state.newBlock.Body.Transactions {
		totalTxsFee[*tx.GetTokenID()] += tx.GetTxFee()
		txType := tx.GetType()
		if txType == common.TxCustomTokenPrivacyType {
			txCustomPrivacy := tx.(*transaction.TxCustomTokenPrivacy)
			totalTxsFee[*txCustomPrivacy.GetTokenID()] = txCustomPrivacy.GetTxFeeToken()
		}
	}
	state.totalTxFee = totalTxsFee

	newBlock := state.newBlock
	merkleRoots := blockchain.Merkle{}.BuildMerkleTreeStore(newBlock.Body.Transactions)
	merkleRoot := &common.Hash{}
	if len(merkleRoots) > 0 {
		merkleRoot = merkleRoots[len(merkleRoots)-1]
	}
	crossTransactionRoot, err := blockchain.CreateMerkleCrossTransaction(newBlock.Body.CrossTransactions)
	if err != nil {
		return err
	}

	txInstructions, err := CreateShardInstructionsFromTransactionAndInstruction(newBlock.Body.Transactions, state.bc, shardID)
	if err != nil {
		return err
	}
	totalInstructions := []string{}
	for _, value := range txInstructions {
		totalInstructions = append(totalInstructions, value...)
	}
	for _, value := range newBlock.Body.Instructions {
		totalInstructions = append(totalInstructions, value...)
	}
	instructionsHash, err := GenerateHashFromStringArray(totalInstructions)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.InstructionsHashError, err)
	}

	tempShardCommitteePubKeys, err := incognitokey.CommitteeKeyListToString(newView.ShardCommittee)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	committeeRoot, err := GenerateHashFromStringArray(tempShardCommitteePubKeys)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.CommitteeHashError, err)
	}
	tempShardPendintValidator, err := incognitokey.CommitteeKeyListToString(newView.ShardPendingValidator)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	pendingValidatorRoot, err := GenerateHashFromStringArray(tempShardPendintValidator)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.PendingValidatorRootError, err)
	}
	stakingTxRoot, err := GenerateHashFromMapStringString(state.stakingTx)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.StakingTxHashError, err)
	}

	flattenTxInsts, err := blockchain.FlattenAndConvertStringInst(txInstructions)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.FlattenAndConvertStringInstError, fmt.Errorf("Instruction from Tx: %+v", err))
	}
	flattenInsts, err := blockchain.FlattenAndConvertStringInst(newBlock.Body.Instructions)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.FlattenAndConvertStringInstError, fmt.Errorf("Instruction from block body: %+v", err))
	}
	insts := append(flattenTxInsts, flattenInsts...) // Order of instructions must be preserved in shardprocess
	instMerkleRoot := blockchain.GetKeccak256MerkleRoot(insts)
	// shard tx root
	_, shardTxMerkleData := blockchain.CreateShardTxRoot2(newBlock.Body.Transactions)

	state.newBlock.Header = ShardHeader{
		Producer:          state.proposer,
		ProducerPubKeyStr: state.proposer,

		ShardID:              shardID,
		Version:              blockchain.SHARD_BLOCK_VERSION,
		PreviousBlockHash:    *state.curView.Block.Hash(),
		Height:               state.curView.Block.GetHeight() + 1,
		Round:                1,
		TimeSlot:             state.createTimeSlot,
		Epoch:                state.newBlockEpoch,
		CrossShardBitMap:     blockchain.CreateCrossShardByteArray(state.newBlock.Body.Transactions, shardID),
		BeaconHeight:         state.newConfirmBeaconHeight,
		BeaconHash:           state.newConfirmBeaconHash,
		TotalTxsFee:          state.totalTxFee,
		ConsensusType:        common.BlsConsensus2,
		Timestamp:            state.createTimeStamp,
		TxRoot:               *merkleRoot,
		ShardTxRoot:          shardTxMerkleData[len(shardTxMerkleData)-1],
		CrossTransactionRoot: *crossTransactionRoot,
		InstructionsRoot:     instructionsHash,
		CommitteeRoot:        committeeRoot,
		PendingValidatorRoot: pendingValidatorRoot,
		StakingTxRoot:        stakingTxRoot,
	}
	copy(newBlock.Header.InstructionMerkleRoot[:], instMerkleRoot)

	state.newBlock.ConsensusHeader = ConsensusHeader{
		TimeSlot:       state.createTimeSlot,
		Proposer:       state.proposer,
		ValidationData: "",
	}

	return nil
}

//==============================Validate Logic===============================
func (s *ShardCoreApp) preValidate() error {
	state := s.ValidateState
	shardID := state.curView.ShardID
	newBlock := state.newView.GetBlock().(*ShardBlock)
	oldBlock := state.curView.GetBlock().(*ShardBlock)

	//check block proposer
	key := incognitokey.CommitteePublicKey{}
	err := key.FromBase58(newBlock.GetBlockProposer())
	if err != nil {
		return err
	}
	if key.GetMiningKeyBase58(common.BlsConsensus2) != state.curView.GetNextProposer(newBlock.GetTimeslot()) {
		return errors.New("Wrong block proposer " + newBlock.GetBlockProposer() + " " + state.curView.GetNextProposer(newBlock.GetTimeslot()))
	}

	// TODO: check block version, should be function which return version base on block height
	if newBlock.Header.Version != blockchain.SHARD_BLOCK_VERSION {
		return blockchain.NewBlockChainError(blockchain.WrongVersionError, fmt.Errorf("Expect newBlock version %+v but get %+v", 1, newBlock.Header.Version))
	}

	if state.isPreSign {
		// get beacon blocks confirmed by proposed block
		if newBlock.Header.BeaconHeight < oldBlock.Header.BeaconHeight {
			return errors.New("Beaconheight is not valid")
		}
		if newBlock.Header.BeaconHeight > oldBlock.Header.BeaconHeight {
			state.isOldBeaconHeight = true
		}
		if newBlock.Header.BeaconHeight > oldBlock.Header.BeaconHeight {
			fmt.Println("beacon ", newBlock.Header.BeaconHeight, oldBlock.Header.BeaconHeight)
			state.isOldBeaconHeight = false
			beaconBlocks := state.bc.GetValidBeaconBlockFromPool()
			if len(beaconBlocks) == int(newBlock.Header.BeaconHeight-oldBlock.Header.BeaconHeight) {
				state.beaconBlocks = beaconBlocks
			} else {
				return errors.New("Not enough beacon to validate")
			}
		}

		// get crossshard blocks confirmed by proposed block
		allConfirmCrossShard := make(map[byte][]*CrossShardBlock)
		for _, beaconBlk := range state.beaconBlocks {
			confirmCrossShard := beaconBlk.GetConfirmedCrossShardBlockToShard()
			for fromShardID, v := range confirmCrossShard[shardID] {
				for _, crossShard := range v {
					allConfirmCrossShard[fromShardID] = append(allConfirmCrossShard[fromShardID], crossShard)
				}
			}
		}
		state.crossShardBlocks = allConfirmCrossShard
		//TODO: get crossshard from pool and check if we have enough to validate

		// get transaction confirmed by proposed block
		for _, tx := range newBlock.Body.Transactions {
			txType := tx.GetType()
			if txType == common.TxCustomTokenPrivacyType {
				state.txsToAdd = append(state.txsToAdd, tx.(*transaction.TxCustomTokenPrivacy))
			}
		}
	}
	return nil
}

//==============================Save Database Logic===========================
func (s *ShardCoreApp) storeDatabase(state *StoreDatabaseState) error {

	return nil
}

func (s *ShardCoreApp) updateNewViewFromBlock(block *ShardBlock) error {
	s.CreateState.newView.Block = block
	return nil
}
