package blockchain

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
)

// NewBlockShard Create New block Shard:
//	1. Identify Beacon State for this Shard Block: Beacon Hash & Beacon Height & Epoch
//		+ Get Beacon Block (B) from Beacon Best State (from Beacon Chain of Shard Node)
//		+ Beacon Block (B) must have the same epoch With New Shard Block (S):
//		+ If Beacon Block (B) have different height previous shard block PS (previous of S)
//		Then Beacon Block (B) epoch greater than Shard Block (S) epoch exact 1 value
//		BUT This only works if Shard Best State have the Beacon Height divisible by epoch
//		+ Ex: 1 epoch has 50 block
//		Example 1:
//			shard block with
//				height 10,
//				epoch: 1,
//				beacon block height: 49
//			then shard block with
//				height 11 must have
//				epoch: 1,
//				beacon block height: must be 49 or 50
//		Example 2:
//			shard block with
//				height 10,
//				epoch: 1,
//				beacon block height: 50
//			then shard block with
//				height is 11 can have 2 option:
//				a. epoch: 1, if beacon block height remain 50
//				b. epoch: 2, and beacon block must in range from 51-100
//				Can have beacon block with height > 100
//	2. Build Shard Block Body:
//		a. Get Cross Transaction from other shard && Build Cross Shard Tx Custom Token Transaction (if exist)
//		b. Get Transactions for New Block
//		c. Process Assign Instructions from Beacon Blocks
//		c. Generate Instructions
//	3. Build Shard Block Essential Data for Header
//	4. Update Cloned ShardBestState with New Shard Block
//	5. Create Root Hash from New Shard Block and updated Clone Shard Beststate Data
func (blockchain *BlockChain) NewBlockShard(curView *ShardBestState, version int, proposer string, round int, start time.Time) (*ShardBlock, error) {
	var (
		transactionsForNewBlock = make([]metadata.Transaction, 0)
		totalTxsFee             = make(map[common.Hash]uint64)
		newShardBlock           = NewShardBlock()
		instructions            = [][]string{}
		isOldBeaconHeight       = false
		tempPrivateKey          = blockchain.config.BlockGen.createTempKeyset()
		shardBestState          = NewShardBestState()
		beaconHeight            = blockchain.BeaconChain.GetFinalView().GetHeight()
		shardID                 = curView.ShardID
	)
	Logger.log.Criticalf("⛏ Creating Shard Block %+v", curView.ShardHeight+1)
	// startTime := time.Now()
	shardPendingValidator, err := incognitokey.CommitteeKeyListToString(curView.ShardPendingValidator)
	if err != nil {
		return nil, err
	}
	currentCommitteePubKeys, err := incognitokey.CommitteeKeyListToString(curView.ShardCommittee)
	if err != nil {
		return nil, err
	}
	//========Verify newShardBlock with previous best state
	// Get Beststate of previous newShardBlock == previous best state
	// Clone best state value into new variable
	// // startStep := time.Now()
	if err := shardBestState.cloneShardBestStateFrom(curView); err != nil {
		return nil, err
	}
	//==========Fetch Beacon Blocks============
	// // startStep = time.Now()
	BLogger.log.Infof("Producing block: %d", curView.ShardHeight+1)
	if beaconHeight-shardBestState.BeaconHeight > MAX_BEACON_BLOCK {
		beaconHeight = shardBestState.BeaconHeight + MAX_BEACON_BLOCK
	}
	beaconHashes, err := rawdbv2.GetBeaconBlockHashByIndex(blockchain.GetBeaconChainDatabase(), beaconHeight)
	if err != nil {
		return nil, err
	}
	beaconHash := beaconHashes[0]
	beaconBlockBytes, err := rawdbv2.GetBeaconBlockByHash(blockchain.GetBeaconChainDatabase(), beaconHash)
	if err != nil {
		return nil, err
	}
	beaconBlock := BeaconBlock{}
	err = json.Unmarshal(beaconBlockBytes, &beaconBlock)
	if err != nil {
		return nil, err
	}
	epoch := beaconBlock.Header.Epoch
	if epoch-shardBestState.Epoch >= 1 {
		beaconHeight = shardBestState.Epoch * blockchain.config.ChainParams.Epoch
		newBeaconHashes, err := rawdbv2.GetBeaconBlockHashByIndex(blockchain.GetBeaconChainDatabase(), beaconHeight)
		if err != nil {
			return nil, err
		}
		newBeaconHash := newBeaconHashes[0]
		copy(beaconHash[:], newBeaconHash.GetBytes())
		epoch = shardBestState.Epoch + 1
	}
	Logger.log.Infof("Get Beacon Block With Height %+v, Shard BestState %+v", beaconHeight, shardBestState.BeaconHeight)
	//Fetch beacon block from height
	beaconBlocks, err := FetchBeaconBlockFromHeight(blockchain.GetBeaconChainDatabase(), shardBestState.BeaconHeight+1, beaconHeight)
	if err != nil {
		return nil, err
	}
	// this  beacon height is already seen by shard best state
	if beaconHeight == shardBestState.BeaconHeight {
		isOldBeaconHeight = true
	}
	//==========Build block body============
	// Get Transaction For new Block
	// Get Cross output coin from other shard && produce cross shard transaction
	crossTransactions := blockchain.config.BlockGen.getCrossShardData(shardID, shardBestState.BeaconHeight, beaconHeight)
	Logger.log.Critical("Cross Transaction: ", crossTransactions)
	// Get Transaction for new block
	// // startStep = time.Now()
	blockCreationLeftOver := curView.BlockMaxCreateTime.Nanoseconds() - time.Since(start).Nanoseconds()
	txsToAddFromBlock, err := blockchain.config.BlockGen.getTransactionForNewBlock(curView, &tempPrivateKey, shardID, beaconBlocks, blockCreationLeftOver, beaconHeight)
	if err != nil {
		return nil, err
	}
	transactionsForNewBlock = append(transactionsForNewBlock, txsToAddFromBlock...)
	// build txs with metadata
	transactionsForNewBlock, err = blockchain.BuildResponseTransactionFromTxsWithMetadata(curView, transactionsForNewBlock, &tempPrivateKey, shardID)
	// process instruction from beacon block
	shardPendingValidator, _, _ = blockchain.processInstructionFromBeacon(curView, beaconBlocks, shardID, newCommitteeChange())
	// Create Instruction
	instructions, _, _, err = blockchain.generateInstruction(curView, shardID, beaconHeight, isOldBeaconHeight, beaconBlocks, shardPendingValidator, currentCommitteePubKeys)
	if err != nil {
		return nil, NewBlockChainError(GenerateInstructionError, err)
	}
	if len(instructions) != 0 {
		Logger.log.Info("Shard Producer: Instruction", instructions)
	}
	newShardBlock.BuildShardBlockBody(instructions, crossTransactions, transactionsForNewBlock)
	//==========Build Essential Header Data=========
	// producer key
	producerKey := proposer
	producerPubKeyStr := proposer

	for _, tx := range newShardBlock.Body.Transactions {
		totalTxsFee[*tx.GetTokenID()] += tx.GetTxFee()
		txType := tx.GetType()
		if txType == common.TxCustomTokenPrivacyType {
			txCustomPrivacy := tx.(*transaction.TxCustomTokenPrivacy)
			totalTxsFee[*txCustomPrivacy.GetTokenID()] = txCustomPrivacy.GetTxFeeToken()
		}
	}
	newShardBlock.Header = ShardHeader{
		Producer:          producerKey, //committeeMiningKeys[producerPosition],
		ProducerPubKeyStr: producerPubKeyStr,
		ShardID:           shardID,
		Version:           version,
		PreviousBlockHash: shardBestState.BestBlockHash,
		Height:            shardBestState.ShardHeight + 1,
		Round:             round,
		Epoch:             epoch,
		CrossShardBitMap:  CreateCrossShardByteArray(newShardBlock.Body.Transactions, shardID),
		BeaconHeight:      beaconHeight,
		BeaconHash:        beaconHash,
		TotalTxsFee:       totalTxsFee,
		ConsensusType:     curView.ConsensusAlgorithm,
	}
	//============Update Shard BestState=============
	// startStep = time.Now()
	newShardBestState, err := shardBestState.updateShardBestState(blockchain, newShardBlock, beaconBlocks, newCommitteeChange())
	if err != nil {
		return nil, err
	}
	//============Build Header=============
	// Build Root Hash for Header
	merkleRoots := Merkle{}.BuildMerkleTreeStore(newShardBlock.Body.Transactions)
	merkleRoot := &common.Hash{}
	if len(merkleRoots) > 0 {
		merkleRoot = merkleRoots[len(merkleRoots)-1]
	}
	crossTransactionRoot, err := CreateMerkleCrossTransaction(newShardBlock.Body.CrossTransactions)
	if err != nil {
		return nil, err
	}
	txInstructions, err := CreateShardInstructionsFromTransactionAndInstruction(newShardBlock.Body.Transactions, blockchain, shardID)
	if err != nil {
		return nil, err
	}
	totalInstructions := []string{}
	for _, value := range txInstructions {
		totalInstructions = append(totalInstructions, value...)
	}
	for _, value := range instructions {
		totalInstructions = append(totalInstructions, value...)
	}
	instructionsHash, err := generateHashFromStringArray(totalInstructions)
	if err != nil {
		return nil, NewBlockChainError(InstructionsHashError, err)
	}
	tempShardCommitteePubKeys, err := incognitokey.CommitteeKeyListToString(newShardBestState.ShardCommittee)
	if err != nil {
		return nil, NewBlockChainError(UnExpectedError, err)
	}
	committeeRoot, err := generateHashFromStringArray(tempShardCommitteePubKeys)
	if err != nil {
		return nil, NewBlockChainError(CommitteeHashError, err)
	}
	tempShardPendintValidator, err := incognitokey.CommitteeKeyListToString(newShardBestState.ShardPendingValidator)
	if err != nil {
		return nil, NewBlockChainError(UnExpectedError, err)
	}
	pendingValidatorRoot, err := generateHashFromStringArray(tempShardPendintValidator)
	if err != nil {
		return nil, NewBlockChainError(PendingValidatorRootError, err)
	}
	stakingTxRoot, err := generateHashFromMapStringString(newShardBestState.StakingTx)
	if err != nil {
		return nil, NewBlockChainError(StakingTxHashError, err)
	}
	// Instruction merkle root
	flattenTxInsts, err := FlattenAndConvertStringInst(txInstructions)
	if err != nil {
		return nil, NewBlockChainError(FlattenAndConvertStringInstError, fmt.Errorf("Instruction from Tx: %+v", err))
	}
	flattenInsts, err := FlattenAndConvertStringInst(instructions)
	if err != nil {
		return nil, NewBlockChainError(FlattenAndConvertStringInstError, fmt.Errorf("Instruction from block body: %+v", err))
	}
	insts := append(flattenTxInsts, flattenInsts...) // Order of instructions must be preserved in shardprocess
	instMerkleRoot := GetKeccak256MerkleRoot(insts)
	// shard tx root
	_, shardTxMerkleData := CreateShardTxRoot(newShardBlock.Body.Transactions)
	// Add Root Hash To Header
	newShardBlock.Header.TxRoot = *merkleRoot
	newShardBlock.Header.ShardTxRoot = shardTxMerkleData[len(shardTxMerkleData)-1]
	newShardBlock.Header.CrossTransactionRoot = *crossTransactionRoot
	newShardBlock.Header.InstructionsRoot = instructionsHash
	newShardBlock.Header.CommitteeRoot = committeeRoot
	newShardBlock.Header.PendingValidatorRoot = pendingValidatorRoot
	newShardBlock.Header.StakingTxRoot = stakingTxRoot
	newShardBlock.Header.Timestamp = start.Unix()
	copy(newShardBlock.Header.InstructionMerkleRoot[:], instMerkleRoot)
	return newShardBlock, nil
}

// getTransactionForNewBlock get transaction for new block
// 1. Get pending transaction from blockgen
// 2. Keep valid tx & Removed error tx
// 3. Build response Transaction For Shard
// 4. Build response Transaction For Beacon
// 5. Return valid transaction from pending, response transactions from shard and beacon
func (blockGenerator *BlockGenerator) getTransactionForNewBlock(curView *ShardBestState, privatekey *privacy.PrivateKey, shardID byte, beaconBlocks []*BeaconBlock, blockCreation int64, beaconHeight uint64) ([]metadata.Transaction, error) {
	txsToAdd, txToRemove, _ := blockGenerator.getPendingTransaction(shardID, beaconBlocks, blockCreation, beaconHeight, curView)
	if len(txsToAdd) == 0 {
		Logger.log.Info("Creating empty block...")
	}
	go blockGenerator.txPool.RemoveTx(txToRemove, false)
	var responseTxsBeacon []metadata.Transaction
	var errInstructions [][]string
	var cError chan error
	cError = make(chan error)
	go func() {
		var err error
		responseTxsBeacon, errInstructions, err = blockGenerator.buildResponseTxsFromBeaconInstructions(curView, beaconBlocks, privatekey, shardID)
		cError <- err
	}()
	nilCount := 0
	for {
		err := <-cError
		if err != nil {
			return nil, err
		}
		nilCount++
		if nilCount == 1 {
			break
		}
	}
	txsToAdd = append(txsToAdd, responseTxsBeacon...)
	if len(errInstructions) > 0 {
		Logger.log.Error("List error instructions, which can not create tx", errInstructions)
	}
	return txsToAdd, nil
}

// buildResponseTxsFromBeaconInstructions builds response txs from beacon instructions
func (blockGenerator *BlockGenerator) buildResponseTxsFromBeaconInstructions(curView *ShardBestState, beaconBlocks []*BeaconBlock, producerPrivateKey *privacy.PrivateKey, shardID byte) ([]metadata.Transaction, [][]string, error) {
	responsedTxs := []metadata.Transaction{}
	responsedHashTxs := []common.Hash{} // capture hash of responsed tx
	errorInstructions := [][]string{}   // capture error instruction -> which instruction can not create tx
	tempAutoStakingM := make(map[uint64]map[string]bool)
	beaconView := blockGenerator.chain.BeaconChain.GetFinalView().(*BeaconBestState)
	for _, beaconBlock := range beaconBlocks {
		autoStaking, ok := tempAutoStakingM[beaconBlock.Header.Height]
		if !ok {
			beaconConsensusStateRootHash, err := blockGenerator.chain.GetBeaconConsensusRootHash(blockGenerator.chain.GetBeaconChainDatabase(), beaconBlock.Header.Height-1)
			if err != nil {
				return []metadata.Transaction{}, errorInstructions, NewBlockChainError(FetchAutoStakingByHeightError, fmt.Errorf("can't get ConsensusStateRootHash of height %+v ,error %+v", beaconBlock.Header.Height, err))
			}
			beaconConsensusStateDB, err := statedb.NewWithPrefixTrie(beaconConsensusStateRootHash, statedb.NewDatabaseAccessWarper(blockGenerator.chain.GetBeaconChainDatabase()))
			if err != nil {
				return []metadata.Transaction{}, errorInstructions, NewBlockChainError(FetchAutoStakingByHeightError, err)
			}
			_, newAutoStaking := statedb.GetRewardReceiverAndAutoStaking(beaconConsensusStateDB, blockGenerator.chain.GetShardIDs())
			tempAutoStakingM[beaconBlock.Header.Height] = newAutoStaking
			autoStaking = newAutoStaking
		}
		for _, l := range beaconBlock.Body.Instructions {
			if l[0] == SwapAction {
				for _, outPublicKeys := range strings.Split(l[2], ",") {
					// If out public key has auto staking then ignore this public key
					res, ok := autoStaking[outPublicKeys]
					if ok && res {
						continue
					}
					tx, err := blockGenerator.buildReturnStakingAmountTx(curView, outPublicKeys, producerPrivateKey, shardID)
					if err != nil {
						if strings.Index(err.Error(), "No staking tx in best state") == -1 {
							Logger.log.Error(err)
						}
						continue
					}
					txHash := *tx.Hash()
					if ok, _ := common.SliceExists(responsedHashTxs, txHash); ok {
						data, _ := json.Marshal(tx)
						Logger.log.Error("Double tx from instruction", l, string(data))
						errorInstructions = append(errorInstructions, l)
						continue
					}
					responsedTxs = append(responsedTxs, tx)
					responsedHashTxs = append(responsedHashTxs, txHash)
				}

			}
			if l[0] == StakeAction || l[0] == RandomAction || l[0] == AssignAction || l[0] == SwapAction {
				continue
			}
			if len(l) <= 2 {
				continue
			}
			metaType, err := strconv.Atoi(l[0])
			if err != nil {
				return nil, nil, err
			}
			var newTx metadata.Transaction
			switch metaType {
			case metadata.IssuingETHRequestMeta:
				if len(l) >= 4 && l[2] == "accepted" {
					newTx, err = blockGenerator.buildETHIssuanceTx(l[3], producerPrivateKey, shardID, curView, beaconView)
				}
			case metadata.IssuingRequestMeta:
				if len(l) >= 4 && l[2] == "accepted" {
					newTx, err = blockGenerator.buildIssuanceTx(l[3], producerPrivateKey, shardID, curView, beaconView)
				}
			case metadata.PDETradeRequestMeta:
				if len(l) >= 4 {
					newTx, err = blockGenerator.buildPDETradeIssuanceTx(l[2], l[3], producerPrivateKey, shardID, curView, beaconView)
				}
			case metadata.PDEWithdrawalRequestMeta:
				if len(l) >= 4 && l[2] == common.PDEWithdrawalAcceptedChainStatus {
					newTx, err = blockGenerator.buildPDEWithdrawalTx(l[3], producerPrivateKey, shardID, curView, beaconView)
				}
			case metadata.PDEContributionMeta:
				if len(l) >= 4 {
					if l[2] == common.PDEContributionRefundChainStatus {
						newTx, err = blockGenerator.buildPDERefundContributionTx(l[3], producerPrivateKey, shardID, curView, beaconView)
					} else if l[2] == common.PDEContributionMatchedNReturnedChainStatus {
						newTx, err = blockGenerator.buildPDEMatchedNReturnedContributionTx(l[3], producerPrivateKey, shardID, curView, beaconView)
					}
				}
			case metadata.PortalCustodianDepositMeta:
				if len(l) >= 4 && l[2] == common.PortalCustodianDepositRefundChainStatus {
					newTx, err = curView.buildPortalRefundCustodianDepositTx(l[3], producerPrivateKey, shardID)
				}
			case metadata.PortalUserRequestPTokenMeta:
				if len(l) >= 4 && l[2] == common.PortalReqPTokensAcceptedChainStatus {
					newTx, err = curView.buildPortalAcceptedRequestPTokensTx(blockGenerator.chain.GetBeaconBestState(), l[3], producerPrivateKey, shardID)
				}
				//custodian withdraw
			case metadata.PortalCustodianWithdrawRequestMeta:
				if len(l) >= 4 && l[2] == common.PortalCustodianWithdrawRequestAcceptedStatus {
					newTx, err = curView.buildPortalCustodianWithdrawRequest(l[3], producerPrivateKey, shardID)
				}
			case metadata.PortalRedeemRequestMeta:
				if len(l) >= 4 && l[2] == common.PortalRedeemRequestRejectedChainStatus {
					newTx, err = curView.buildPortalRejectedRedeemRequestTx(blockGenerator.chain.GetBeaconBestState(), l[3], producerPrivateKey, shardID)
				}
				//liquidation: redeem ptoken
			case metadata.PortalRedeemLiquidateExchangeRatesMeta:
				if len(l) >= 4 && l[2] == common.PortalRedeemLiquidateExchangeRatesSuccessChainStatus {
					newTx, err = curView.buildPortalRedeemLiquidateExchangeRatesRequestTx(l[3], producerPrivateKey, shardID)
				}
			case metadata.PortalLiquidateCustodianMeta:
				if len(l) >= 4 && l[2] == common.PortalLiquidateCustodianSuccessChainStatus {
					newTx, err = curView.buildPortalLiquidateCustodianResponseTx(l[3], producerPrivateKey, shardID)
				}
			case metadata.PortalRequestWithdrawRewardMeta:
				if len(l) >= 4 && l[2] == common.PortalReqWithdrawRewardAcceptedChainStatus {
					newTx, err = curView.buildPortalAcceptedWithdrawRewardTx(blockGenerator.chain.GetBeaconBestState(), l[3], producerPrivateKey, shardID)
				}
				//liquidation: custodian deposit
			case metadata.PortalLiquidationCustodianDepositMeta:
				if len(l) >= 4 && l[2] == common.PortalLiquidationCustodianDepositRejectedChainStatus {
					newTx, err = curView.buildPortalLiquidationCustodianDepositReject(l[3], producerPrivateKey, shardID)
				}
			default:
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			if newTx != nil {
				newTxHash := *newTx.Hash()
				if ok, _ := common.SliceExists(responsedHashTxs, newTxHash); ok {
					data, _ := json.Marshal(newTx)
					Logger.log.Error("Double tx from instruction", l, string(data))
					errorInstructions = append(errorInstructions, l)
					//continue
				}
				responsedTxs = append(responsedTxs, newTx)
				responsedHashTxs = append(responsedHashTxs, newTxHash)
			}
		}
	}
	return responsedTxs, errorInstructions, nil
}

// Process Instruction From Beacon Blocks:
//	- Assign Instruction: get more pending validator from beacon and return new list of pending validator
//	 + ["assign" "shardCandidate1,shardCandidate2,..." "shard" "{shardID}"]
//	- stake instruction format
//	 + ["stake", "pubkey1,pubkey2,..." "shard" "txStake1,txStake2,..." "rewardReceiver1,rewardReceiver2,..." flag]
//	 + ["stake", "pubkey1,pubkey2,..." "beacon" "txStake1,txStake2,..." "rewardReceiver1,rewardReceiver2,..." flag]
func (blockchain *BlockChain) processInstructionFromBeacon(curView *ShardBestState, beaconBlocks []*BeaconBlock, shardID byte, committeeChange *committeeChange) ([]string, []string, map[string]string) {
	newShardPendingValidator := []string{}
	shardPendingValidator := []string{}
	if curView != nil {
		var err error
		shardPendingValidator, err = incognitokey.CommitteeKeyListToString(curView.ShardPendingValidator)
		if err != nil {
			panic(err)
		}
	}

	assignInstructions := [][]string{}
	stakingTx := make(map[string]string)
	for _, beaconBlock := range beaconBlocks {
		for _, l := range beaconBlock.Body.Instructions {
			// Process Assign Instruction
			if l[0] == AssignAction && l[2] == "shard" {
				if strings.Compare(l[3], strconv.Itoa(int(shardID))) == 0 {
					tempNewShardPendingValidator := strings.Split(l[1], ",")
					shardPendingValidator = append(shardPendingValidator, tempNewShardPendingValidator...)
					newShardPendingValidator = append(newShardPendingValidator, tempNewShardPendingValidator...)
					assignInstructions = append(assignInstructions, l)
				}
			}
			// Get Staking Tx
			// assume that stake instruction already been validated by beacon committee
			if l[0] == StakeAction && l[2] == "beacon" {
				beacon := strings.Split(l[1], ",")
				newBeaconCandidates := []string{}
				newBeaconCandidates = append(newBeaconCandidates, beacon...)
				if len(l) == 6 {
					for i, v := range strings.Split(l[3], ",") {
						txHash, err := common.Hash{}.NewHashFromStr(v)
						if err != nil {
							continue
						}
						_, _, _, err = blockchain.GetTransactionByHashWithShardID(*txHash, shardID)
						if err != nil {
							continue
						}
						// if transaction belong to this shard then add to shard beststate
						stakingTx[newBeaconCandidates[i]] = v
					}
				}
			}
			if l[0] == StakeAction && l[2] == "shard" {
				shard := strings.Split(l[1], ",")
				newShardCandidates := []string{}
				newShardCandidates = append(newShardCandidates, shard...)
				if len(l) == 6 {
					for i, v := range strings.Split(l[3], ",") {
						txHash, err := common.Hash{}.NewHashFromStr(v)
						if err != nil {
							continue
						}
						_, _, _, err = blockchain.GetTransactionByHashWithShardID(*txHash, shardID)
						if err != nil {
							continue
						}
						// if transaction belong to this shard then add to shard beststate
						stakingTx[newShardCandidates[i]] = v
					}
				}
			}
		}
	}
	return shardPendingValidator, newShardPendingValidator, stakingTx
}

//	Generate Instruction generate instruction for new shard block
//	- Swap: at the end of beacon epoch
//	- Brigde: at the end of beacon epoch
//	Return params:
//	#1: instruction list
//	#2: shardpendingvalidator
//	#3: shardcommittee
//	#4: error
func (blockchain *BlockChain) generateInstruction(view *ShardBestState, shardID byte, beaconHeight uint64, isOldBeaconHeight bool, beaconBlocks []*BeaconBlock, shardPendingValidator []string, shardCommittee []string) ([][]string, []string, []string, error) {
	var (
		instructions          = [][]string{}
		bridgeSwapConfirmInst = []string{}
		swapInstruction       = []string{}
		// err                   error
	)
	// if this beacon height has been seen already then DO NOT generate any more instruction
	if beaconHeight%blockchain.config.ChainParams.Epoch == 0 && isOldBeaconHeight == false {
		// TODO: 0xmerman
		fixedProducerShardValidators := shardCommittee[:NumberOfFixedBlockValidators]
		shardCommittee = shardCommittee[NumberOfFixedBlockValidators:]

		Logger.log.Info("ShardPendingValidator", shardPendingValidator)
		Logger.log.Info("ShardCommittee", shardCommittee)
		Logger.log.Info("MaxShardCommitteeSize", view.MaxShardCommitteeSize)
		Logger.log.Info("ShardID", shardID)
		beaconSlashRootHash, err := blockchain.GetBeaconSlashRootHash(blockchain.GetBeaconChainDatabase(), beaconHeight)
		if err != nil {
			return instructions, shardPendingValidator, shardCommittee, err
		}
		beaconSlashStateDB, err := statedb.NewWithPrefixTrie(beaconSlashRootHash, statedb.NewDatabaseAccessWarper(blockchain.GetBeaconChainDatabase()))
		if err != nil {
			return instructions, shardPendingValidator, shardCommittee, err
		}
		producersBlackList, err := blockchain.getUpdatedProducersBlackList(beaconSlashStateDB, false, int(shardID), shardCommittee, beaconHeight)
		if err != nil {
			Logger.log.Error(err)
			return instructions, shardPendingValidator, shardCommittee, err
		}

		maxShardCommitteeSize := view.MaxShardCommitteeSize - NumberOfFixedBlockValidators
		var minShardCommitteeSize int
		if view.MinShardCommitteeSize-NumberOfFixedBlockValidators < 0 {
			minShardCommitteeSize = 0
		} else {
			minShardCommitteeSize = view.MinShardCommitteeSize - NumberOfFixedBlockValidators
		}

		badProducersWithPunishment := blockchain.buildBadProducersWithPunishment(false, int(shardID), shardCommittee)
		swapInstruction, shardPendingValidator, shardCommittee, err = CreateSwapAction(shardPendingValidator, shardCommittee, maxShardCommitteeSize, minShardCommitteeSize, shardID, producersBlackList, badProducersWithPunishment, blockchain.config.ChainParams.Offset, blockchain.config.ChainParams.SwapOffset)
		if err != nil {
			Logger.log.Error(err)
			return instructions, shardPendingValidator, shardCommittee, err
		}
		//TODO: 0xmerman
		//TODO: duybao fixed
		shardCommittee = append(fixedProducerShardValidators, shardCommittee...)
		// NOTE: shardCommittee must be finalized before building Bridge instruction here
		// shardCommittee must include all producers and validators in the right order
		// Generate instruction storing merkle root of validators pubkey and send to beacon
		bridgeID := byte(common.BridgeShardID)
		if shardID == bridgeID && committeeChanged(swapInstruction) {
			blockHeight := view.ShardHeight + 1
			bridgeSwapConfirmInst, err = buildBridgeSwapConfirmInstruction(shardCommittee, blockHeight)
			if err != nil {
				BLogger.log.Error(err)
				return instructions, shardPendingValidator, shardCommittee, err
			}
			BLogger.log.Infof("Add Bridge swap inst in ShardID %+v block %d", shardID, blockHeight)
		}
	}
	if len(swapInstruction) > 0 {
		instructions = append(instructions, swapInstruction)
	}
	if len(bridgeSwapConfirmInst) > 0 {
		instructions = append(instructions, bridgeSwapConfirmInst)
		Logger.log.Infof("Build bridge swap confirm inst: %s \n", bridgeSwapConfirmInst)
	}
	// Pick BurningConfirm inst and save to bridge block
	bridgeID := byte(common.BridgeShardID)
	if shardID == bridgeID {
		prevBlock := view.BestBlock
		height := view.ShardHeight + 1
		confirmInsts := pickBurningConfirmInstruction(beaconBlocks, height)
		if len(confirmInsts) > 0 {
			bid := []uint64{}
			for _, b := range beaconBlocks {
				bid = append(bid, b.Header.Height)
			}
			Logger.log.Infof("Picked burning confirm inst: %s %d %v\n", confirmInsts, prevBlock.Header.Height+1, bid)
			instructions = append(instructions, confirmInsts...)
		}
	}
	return instructions, shardPendingValidator, shardCommittee, nil
}

// getCrossShardData get cross shard data from cross shard block
//  1. Get Cross Shard Block and Validate
//	  a. Get Valid Cross Shard Block from Cross Shard Pool
//	  b. Get Current Cross Shard State: Last Cross Shard Block From other Shard (FS) to this shard (TS) (Ex: last cross shard block from Shard 0 to Shard 1)
//	  c. Get Next Cross Shard Block Height from other Shard (FS) to this shard (TS)
//     + Using FetchCrossShardNextHeight function in Database to determine next block height
//	  d. Fetch Other Shard (FS) Committee at Next Cross Shard Block Height for Validation
//  2. Validate
//	  a. Get Next Cross Shard Height from Database
//	  b. Cross Shard Block Height is Next Cross Shard Height from Database (if miss Cross Shard Block according to beacon bytemap then stop discard the rest)
//	  c. Verify Cross Shard Block Signature
//  3. After validation:
//	  - Process valid block to extract:
//	   + Cross output coin
//	   + Cross Normal Token
func (blockGenerator *BlockGenerator) getCrossShardData(toShard byte, lastBeaconHeight uint64, currentBeaconHeight uint64) map[byte][]CrossTransaction {
	crossTransactions := make(map[byte][]CrossTransaction)
	// get cross shard block
	var allCrossShardBlock = make([][]*CrossShardBlock, blockGenerator.chain.config.ChainParams.ActiveShards)
	for sid, v := range blockGenerator.syncker.GetCrossShardBlocksForShardProducer(toShard) {
		for _, b := range v {
			allCrossShardBlock[sid] = append(allCrossShardBlock[sid], b.(*CrossShardBlock))
		}
	}
	// allCrossShardBlock => already short
	for _, crossShardBlock := range allCrossShardBlock {
		for _, blk := range crossShardBlock {
			crossTransaction := CrossTransaction{
				OutputCoin:       blk.CrossOutputCoin,
				TokenPrivacyData: blk.CrossTxTokenPrivacyData,
				BlockHash:        *blk.Hash(),
				BlockHeight:      blk.Header.Height,
			}
			crossTransactions[blk.Header.ShardID] = append(crossTransactions[blk.Header.ShardID], crossTransaction)
		}
	}
	return crossTransactions
}

/*
	Verify Transaction with these condition: defined in mempool.go
*/
func (blockGenerator *BlockGenerator) getPendingTransaction(
	shardID byte,
	beaconBlocks []*BeaconBlock,
	blockCreationTimeLeftOver int64,
	beaconHeight uint64,
	curView *ShardBestState,
) (txsToAdd []metadata.Transaction, txToRemove []metadata.Transaction, totalFee uint64) {
	spareTime := SpareTime * time.Millisecond
	maxBlockCreationTimeLeftTime := blockCreationTimeLeftOver - spareTime.Nanoseconds()
	startTime := time.Now()
	sourceTxns := blockGenerator.GetPendingTxsV2()
	var elasped int64
	Logger.log.Info("Number of transaction get from Block Generator: ", len(sourceTxns))
	isEmpty := blockGenerator.chain.config.TempTxPool.EmptyPool()
	if !isEmpty {
		return []metadata.Transaction{}, []metadata.Transaction{}, 0
	}
	currentSize := uint64(0)
	preparedTxForNewBlock := []metadata.Transaction{}
	for _, tx := range sourceTxns {
		tempSize := tx.GetTxActualSize()
		if currentSize+tempSize >= common.MaxBlockSize {
			break
		}
		preparedTxForNewBlock = append(preparedTxForNewBlock, tx)
		elasped = time.Since(startTime).Nanoseconds()
		if elasped >= maxBlockCreationTimeLeftTime {
			Logger.log.Info("Shard Producer/Elapsed, Break: ", elasped)
			break
		}
	}
	listBatchTxs := []metadata.Transaction{}
	for index, tx := range preparedTxForNewBlock {
		elasped = time.Since(startTime).Nanoseconds()
		if elasped >= maxBlockCreationTimeLeftTime {
			Logger.log.Info("Shard Producer/Elapsed, Break: ", elasped)
			break
		}
		listBatchTxs = append(listBatchTxs, tx)
		if ((index+1)%TransactionBatchSize == 0) || (index == len(preparedTxForNewBlock)-1) {
			tempTxDesc, err := blockGenerator.chain.config.TempTxPool.MaybeAcceptBatchTransactionForBlockProducing(shardID, listBatchTxs, int64(beaconHeight), curView)
			if err != nil {
				Logger.log.Errorf("SHARD %+v | Verify Batch Transaction for new block error %+v", shardID, err)
				for _, tx2 := range listBatchTxs {
					if blockGenerator.chain.config.TempTxPool.HaveTransaction(tx2.Hash()) {
						continue
					}
					txShardID := common.GetShardIDFromLastByte(tx2.GetSenderAddrLastByte())
					if txShardID != shardID {
						continue
					}
					tempTxDesc, err := blockGenerator.chain.config.TempTxPool.MaybeAcceptTransactionForBlockProducing(tx2, int64(beaconHeight), curView)
					if err != nil {
						txToRemove = append(txToRemove, tx2)
						continue
					}
					tempTx := tempTxDesc.Tx
					totalFee += tempTx.GetTxFee()
					tempSize := tempTx.GetTxActualSize()
					if currentSize+tempSize >= common.MaxBlockSize {
						break
					}
					currentSize += tempSize
					txsToAdd = append(txsToAdd, tempTx)
				}
			}
			for _, tempToAddTxDesc := range tempTxDesc {
				tempTx := tempToAddTxDesc.Tx
				totalFee += tempTx.GetTxFee()
				tempSize := tempTx.GetTxActualSize()
				if currentSize+tempSize >= common.MaxBlockSize {
					break
				}
				currentSize += tempSize
				txsToAdd = append(txsToAdd, tempTx)
			}
			// reset list batch and add new txs
			listBatchTxs = []metadata.Transaction{}

		} else {
			continue
		}
	}
	Logger.log.Criticalf(" 🔎 %+v transactions for New Block from pool \n", len(txsToAdd))
	blockGenerator.chain.config.TempTxPool.EmptyPool()
	return txsToAdd, txToRemove, totalFee
}

func (blockGenerator *BlockGenerator) createTempKeyset() privacy.PrivateKey {
	rand.Seed(time.Now().UnixNano())
	seed := make([]byte, 16)
	rand.Read(seed)
	return privacy.GeneratePrivateKey(seed)
}

// committeeChanged checks if swap instructions really changed the committee list
// (not just empty swap instruction)
func committeeChanged(swap []string) bool {
	if len(swap) < 3 {
		return false
	}

	in := swap[1]
	out := swap[2]
	return len(in) > 0 || len(out) > 0
}
