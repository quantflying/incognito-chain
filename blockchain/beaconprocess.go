package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain/btc"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/pubsub"
	"github.com/pkg/errors"
)

// VerifyPreSignBeaconBlock should receives block in consensus round
// It verify validity of this function before sign it
// This should be verify in the first round of consensus
//	Step:
//	1. Verify Pre proccessing data
//	2. Retrieve beststate for new block, store in local variable
//	3. Update: process local beststate with new block
//	4. Verify Post processing: updated local beststate and newblock
//	Return:
//	- No error: valid and can be sign
//	- Error: invalid new block
func (blockchain *BlockChain) VerifyPreSignBeaconBlock(beaconBlock *BeaconBlock, isPreSign bool) error {
	//get view that block link to
	preHash := beaconBlock.Header.PreviousBlockHash
	view := blockchain.BeaconChain.GetViewByHash(preHash)
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if view == nil {
		blockchain.config.Syncker.SyncMissingBeaconBlock(ctx, "", preHash)
	}

	for {
		select {
		case <-ctx.Done():
			return errors.New(fmt.Sprintf("BeaconBlock %v link to wrong view (%s)", beaconBlock.GetHeight(), preHash.String()))
		default:
			view = blockchain.BeaconChain.GetViewByHash(preHash)
			if view != nil {
				goto CONTINUE_VERIFY
			}
			time.Sleep(time.Second)
		}
	}
CONTINUE_VERIFY:

	curView := view.(*BeaconBestState)
	// Verify block only
	Logger.log.Infof("BEACON | Verify block for signing process %d, with hash %+v", beaconBlock.Header.Height, *beaconBlock.Hash())
	committeeChange := newCommitteeChange()
	if err := blockchain.verifyPreProcessingBeaconBlock(curView, beaconBlock, isPreSign); err != nil {
		return err
	}
	// Verify block with previous best state
	// not verify agg signature in this function
	if err := curView.verifyBestStateWithBeaconBlock(blockchain, beaconBlock, false, blockchain.config.ChainParams.Epoch); err != nil {
		return err
	}
	// Update best state with new block
	newBestState, err := curView.updateBeaconBestState(beaconBlock, blockchain.config.ChainParams.Epoch, blockchain.config.ChainParams.AssignOffset, blockchain.config.ChainParams.RandomTime, committeeChange)
	if err != nil {
		return err
	}
	// Post verififcation: verify new beaconstate with corresponding block
	if err := newBestState.verifyPostProcessingBeaconBlock(beaconBlock, blockchain.config.RandomClient); err != nil {
		return err
	}
	Logger.log.Infof("BEACON | Block %d, with hash %+v is VALID to be 🖊 signed", beaconBlock.Header.Height, *beaconBlock.Hash())
	return nil
}

func (blockchain *BlockChain) InsertBeaconBlock(beaconBlock *BeaconBlock, shouldValidate bool) error {
	blockHash := beaconBlock.Hash()
	Logger.log.Infof("BEACON | InsertBeaconBlock  %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	blockchain.BeaconChain.insertLock.Lock()
	defer blockchain.BeaconChain.insertLock.Unlock()
	startTimeStoreBeaconBlock := time.Now()
	committeeChange := newCommitteeChange()

	//check view if exited
	checkView := blockchain.BeaconChain.GetViewByHash(*beaconBlock.Hash())
	if checkView != nil {
		return nil
	}

	//get view that block link to
	preHash := beaconBlock.Header.PreviousBlockHash
	preView := blockchain.BeaconChain.GetViewByHash(preHash)
	if preView == nil {
		return errors.New(fmt.Sprintf("BeaconBlock %v link to wrong view (%s)", beaconBlock.GetHeight(), preHash.String()))
	}
	curView := preView.(*BeaconBestState)

	if beaconBlock.Header.Height != curView.BeaconHeight+1 {
		return errors.New("Not expected height")
	}

	Logger.log.Debugf("BEACON | Begin Insert new Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	if shouldValidate {
		Logger.log.Debugf("BEACON | Verify Pre Processing, Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
		if err := blockchain.verifyPreProcessingBeaconBlock(curView, beaconBlock, false); err != nil {
			return err
		}
	} else {
		Logger.log.Debugf("BEACON | SKIP Verify Pre Processing, Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	}
	// Verify beaconBlock with previous best state
	if shouldValidate {
		Logger.log.Debugf("BEACON | Verify Best State With Beacon Block, Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
		// Verify beaconBlock with previous best state
		if err := curView.verifyBestStateWithBeaconBlock(blockchain, beaconBlock, true, blockchain.config.ChainParams.Epoch); err != nil {
			return err
		}
		if err := blockchain.config.ConsensusEngine.ValidateBlockCommitteSig(beaconBlock, curView.BeaconCommittee); err != nil {
			return err
		}
	} else {
		Logger.log.Debugf("BEACON | SKIP Verify Best State With Beacon Block, Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	}
	// Backup beststate
	err := rawdbv2.CleanUpPreviousBeaconBestState(blockchain.GetBeaconChainDatabase())
	if err != nil {
		return NewBlockChainError(CleanBackUpError, err)
	}

	// process for slashing, make sure this one is called before update best state
	// since we'd like to process with old committee, not updated committee
	slashErr := blockchain.processForSlashing(curView.slashStateDB, beaconBlock)
	if slashErr != nil {
		Logger.log.Errorf("Failed to process slashing with error: %+v", NewBlockChainError(ProcessSlashingError, slashErr))
	}
	// snapshot current beacon committee and shard committee
	snapshotBeaconCommittee, snapshotAllShardCommittee, err := snapshotCommittee(curView.BeaconCommittee, curView.ShardCommittee)
	if err != nil {
		return NewBlockChainError(SnapshotCommitteeError, err)
	}
	_, snapshotAllShardPending, err := snapshotCommittee([]incognitokey.CommitteePublicKey{}, curView.ShardPendingValidator)
	if err != nil {
		return NewBlockChainError(SnapshotCommitteeError, err)
	}

	snapshotShardWaiting := append([]incognitokey.CommitteePublicKey{}, curView.CandidateShardWaitingForNextRandom...)
	snapshotShardWaiting = append(snapshotShardWaiting, curView.CandidateBeaconWaitingForCurrentRandom...)

	snapshotRewardReceiver, err := snapshotRewardReceiver(curView.RewardReceiver)
	if err != nil {
		return NewBlockChainError(SnapshotRewardReceiverError, err)
	}
	Logger.log.Debugf("BEACON | Update BestState With Beacon Block, Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	// Update best state with new beaconBlock

	newBestState, err := curView.updateBeaconBestState(beaconBlock, blockchain.config.ChainParams.Epoch, blockchain.config.ChainParams.AssignOffset, blockchain.config.ChainParams.RandomTime, committeeChange)
	if err != nil {
		return err
	}
	// updateNumOfBlocksByProducers updates number of blocks produced by producers
	newBestState.updateNumOfBlocksByProducers(beaconBlock, blockchain.config.ChainParams.Epoch)

	newBeaconCommittee, newAllShardCommittee, err := snapshotCommittee(newBestState.BeaconCommittee, newBestState.ShardCommittee)
	if err != nil {
		return NewBlockChainError(SnapshotCommitteeError, err)
	}
	_, newAllShardPending, err := snapshotCommittee([]incognitokey.CommitteePublicKey{}, newBestState.ShardPendingValidator)
	if err != nil {
		return NewBlockChainError(SnapshotCommitteeError, err)
	}

	notifyHighway := false

	newShardWaiting := append([]incognitokey.CommitteePublicKey{}, newBestState.CandidateShardWaitingForNextRandom...)
	newShardWaiting = append(newShardWaiting, newBestState.CandidateBeaconWaitingForCurrentRandom...)

	isChanged := !reflect.DeepEqual(snapshotBeaconCommittee, newBeaconCommittee)
	if isChanged {
		go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
		notifyHighway = true
	}

	isChanged = !reflect.DeepEqual(snapshotShardWaiting, newShardWaiting)
	if isChanged {
		go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
	}
	//Check shard-pending
	for shardID, committee := range newAllShardPending {
		if _, ok := snapshotAllShardPending[shardID]; ok {
			isChanged := !reflect.DeepEqual(snapshotAllShardPending[shardID], committee)
			if isChanged {
				go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
				notifyHighway = true
			}
		} else {
			go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
			notifyHighway = true
		}
	}
	//Check shard-committee
	for shardID, committee := range newAllShardCommittee {
		if _, ok := snapshotAllShardCommittee[shardID]; ok {
			isChanged := !reflect.DeepEqual(snapshotAllShardCommittee[shardID], committee)
			if isChanged {
				go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
				notifyHighway = true
			}
		} else {
			go blockchain.config.ConsensusEngine.CommitteeChange(common.BeaconChainKey)
			notifyHighway = true
		}
	}

	if shouldValidate {
		Logger.log.Debugf("BEACON | Verify Post Processing Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
		// Post verification: verify new beacon best state with corresponding beacon block
		if err := newBestState.verifyPostProcessingBeaconBlock(beaconBlock, blockchain.config.RandomClient); err != nil {
			return err
		}
	} else {
		Logger.log.Debugf("BEACON | SKIP Verify Post Processing Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	}

	Logger.log.Infof("BEACON | Process Store Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, blockHash)
	if err := blockchain.processStoreBeaconBlock(newBestState, beaconBlock, snapshotBeaconCommittee, snapshotAllShardCommittee, snapshotRewardReceiver, committeeChange); err != nil {
		return err
	}

	// go metrics.AnalyzeTimeSeriesMetricDataWithTime(map[string]interface{}{
	// 	metrics.Measurement:      metrics.NumOfBlockInsertToChain,
	// 	metrics.MeasurementValue: float64(1),
	// 	metrics.Tag:              metrics.ShardIDTag,
	// 	metrics.TagValue:         metrics.Beacon,
	// 	metrics.Time:             beaconBlock.Header.Timestamp,
	// })
	// if beaconBlock.Header.Height > 2 {
	// 	go metrics.AnalyzeTimeSeriesMetricDataWithTime(map[string]interface{}{
	// 		metrics.Measurement:      metrics.NumOfRoundPerBlock,
	// 		metrics.MeasurementValue: float64(beaconBlock.Header.Round),
	// 		metrics.Tag:              metrics.ShardIDTag,
	// 		metrics.TagValue:         metrics.Beacon,
	// 		metrics.Time:             beaconBlock.Header.Timestamp,
	// 	})
	// }
	Logger.log.Infof("BEACON | Finish Insert new Beacon Block %+v, with hash %+v", beaconBlock.Header.Height, *beaconBlock.Hash())
	if beaconBlock.Header.Height%50 == 0 {
		BLogger.log.Debugf("Inserted beacon height: %d", beaconBlock.Header.Height)
	}
	go blockchain.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.NewBeaconBlockTopic, beaconBlock))
	go blockchain.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.BeaconBeststateTopic, newBestState))

	// For masternode: broadcast new committee to highways
	if notifyHighway {
		go blockchain.config.Highway.BroadcastCommittee(
			blockchain.config.ChainParams.Epoch,
			newBeaconCommittee,
			newAllShardCommittee,
			newAllShardPending,
		)
	}
	beaconInsertBlockTimer.UpdateSince(startTimeStoreBeaconBlock)
	return nil
}

// updateNumOfBlocksByProducers updates number of blocks produced by producers
func (beaconBestState *BeaconBestState) updateNumOfBlocksByProducers(beaconBlock *BeaconBlock, chainParamEpoch uint64) {
	producer := beaconBlock.GetProducerPubKeyStr()
	if beaconBlock.GetHeight()%chainParamEpoch == 1 {
		beaconBestState.NumOfBlocksByProducers = map[string]uint64{
			producer: 1,
		}
	}
	// Update number of blocks produced by producers in epoch
	numOfBlks, found := beaconBestState.NumOfBlocksByProducers[producer]
	if !found {
		beaconBestState.NumOfBlocksByProducers[producer] = 1
	} else {
		beaconBestState.NumOfBlocksByProducers[producer] = numOfBlks + 1
	}
}

/*
	VerifyPreProcessingBeaconBlock
	This function DOES NOT verify new block with best state
	DO NOT USE THIS with GENESIS BLOCK
	- Producer sanity data
	- Version: compatible with predefined version
	- Previous Block exist in database, fetch previous block by previous hash of new beacon block
	- Check new beacon block height is equal to previous block height + 1
	- Epoch = blockHeight % Epoch == 1 ? Previous Block Epoch + 1 : Previous Block Epoch
	- Timestamp of new beacon block is greater than previous beacon block timestamp
	- ShardStateHash: rebuild shard state hash from shard state body and compare with shard state hash in block header
	- InstructionHash: rebuild instruction hash from instruction body and compare with instruction hash in block header
	- InstructionMerkleRoot: rebuild instruction merkle root from instruction body and compare with instruction merkle root in block header
	- If verify block for signing then verifyPreProcessingBeaconBlockForSigning
*/
func (blockchain *BlockChain) verifyPreProcessingBeaconBlock(curView *BeaconBestState, beaconBlock *BeaconBlock, isPreSign bool) error {
	// if len(beaconBlock.Header.Producer) == 0 {
	// 	return NewBlockChainError(ProducerError, fmt.Errorf("Expect has length 66 but get %+v", len(beaconBlock.Header.Producer)))
	// }
	startTimeVerifyPreProcessingBeaconBlock := time.Now()
	// Verify parent hash exist or not
	previousBlockHash := beaconBlock.Header.PreviousBlockHash
	parentBlockBytes, err := rawdbv2.GetBeaconBlockByHash(blockchain.GetBeaconChainDatabase(), previousBlockHash)
	if err != nil {
		return NewBlockChainError(FetchBeaconBlockError, err)
	}

	previousBeaconBlock := NewBeaconBlock()
	err = json.Unmarshal(parentBlockBytes, previousBeaconBlock)
	if err != nil {
		return NewBlockChainError(UnmashallJsonBeaconBlockError, fmt.Errorf("Failed to unmarshall parent block of block height %+v", beaconBlock.Header.Height))
	}
	// Verify block height with parent block
	if previousBeaconBlock.Header.Height+1 != beaconBlock.Header.Height {
		return NewBlockChainError(WrongBlockHeightError, fmt.Errorf("Expect receive beacon block height %+v but get %+v", previousBeaconBlock.Header.Height+1, beaconBlock.Header.Height))
	}
	// Verify epoch with parent block
	if (beaconBlock.Header.Height != 1) && (beaconBlock.Header.Height%blockchain.config.ChainParams.Epoch == 1) && (previousBeaconBlock.Header.Epoch != beaconBlock.Header.Epoch-1) {
		return NewBlockChainError(WrongEpochError, fmt.Errorf("Expect receive beacon block epoch %+v greater than previous block epoch %+v, 1 value", beaconBlock.Header.Epoch, previousBeaconBlock.Header.Epoch))
	}
	// Verify timestamp with parent block
	if beaconBlock.Header.Timestamp <= previousBeaconBlock.Header.Timestamp {
		return NewBlockChainError(WrongTimestampError, fmt.Errorf("Expect receive beacon block with timestamp %+v greater than previous block timestamp %+v", beaconBlock.Header.Timestamp, previousBeaconBlock.Header.Timestamp))
	}
	if !verifyHashFromShardState(beaconBlock.Body.ShardState, beaconBlock.Header.ShardStateHash) {
		return NewBlockChainError(ShardStateHashError, fmt.Errorf("Expect shard state hash to be %+v", beaconBlock.Header.ShardStateHash))
	}
	tempInstructionArr := []string{}
	for _, strs := range beaconBlock.Body.Instructions {
		tempInstructionArr = append(tempInstructionArr, strs...)
	}
	if hash, ok := verifyHashFromStringArray(tempInstructionArr, beaconBlock.Header.InstructionHash); !ok {
		return NewBlockChainError(InstructionHashError, fmt.Errorf("Expect instruction hash to be %+v but get %+v", beaconBlock.Header.InstructionHash, hash))
	}
	// Shard state must in right format
	// state[i].Height must less than state[i+1].Height and state[i+1].Height - state[i].Height = 1
	for _, shardStates := range beaconBlock.Body.ShardState {
		for i := 0; i < len(shardStates)-2; i++ {
			if shardStates[i+1].Height-shardStates[i].Height != 1 {
				return NewBlockChainError(ShardStateError, fmt.Errorf("Expect Shard State Height to be in the right format, %+v, %+v", shardStates[i+1].Height, shardStates[i].Height))
			}
		}
	}
	// Check if InstructionMerkleRoot is the root of merkle tree containing all instructions in this block
	flattenInsts, err := FlattenAndConvertStringInst(beaconBlock.Body.Instructions)
	if err != nil {
		return NewBlockChainError(FlattenAndConvertStringInstError, err)
	}
	root := GetKeccak256MerkleRoot(flattenInsts)
	if !bytes.Equal(root, beaconBlock.Header.InstructionMerkleRoot[:]) {
		return NewBlockChainError(FlattenAndConvertStringInstError, fmt.Errorf("Expect Instruction Merkle Root in Beacon Block Header to be %+v but get %+v", string(beaconBlock.Header.InstructionMerkleRoot[:]), string(root)))
	}
	// if pool does not have one of needed block, fail to verify
	beaconVerifyPreprocesingTimer.UpdateSince(startTimeVerifyPreProcessingBeaconBlock)
	if isPreSign {
		if err := blockchain.verifyPreProcessingBeaconBlockForSigning(curView, beaconBlock); err != nil {
			return err
		}
	}
	return nil
}

// verifyPreProcessingBeaconBlockForSigning verify block before signing
// Must pass these following condition:
//  - Rebuild Reward By Epoch Instruction
//  - Get All Shard To Beacon Block in Shard To Beacon Pool
//  - For all Shard To Beacon Blocks in each Shard
//	  + Compare all shard height of shard states in body and these Shard To Beacon Blocks (got from pool)
//		* Must be have the same range of height
//		* Compare CrossShardBitMap of each Shard To Beacon Block and Shard State in New Beacon Block Body
//	  + After finish comparing these shard to beacon blocks with shard states in new beacon block body
//		* Verifying Shard To Beacon Block Agg Signature
//		* Only accept block in one epoch
//	  + Get Instruction from these Shard To Beacon Blocks:
//		* Stake Instruction
//		* Swap Instruction
//		* Bridge Instruction
//		* Block Reward Instruction
//	+ Generate Instruction Hash from all recently got instructions
//	+ Compare just created Instruction Hash with Instruction Hash In Beacon Header
func (blockchain *BlockChain) verifyPreProcessingBeaconBlockForSigning(curView *BeaconBestState, beaconBlock *BeaconBlock) error {
	var err error
	startTimeVerifyPreProcessingBeaconBlockForSigning := time.Now()
	rewardByEpochInstruction := [][]string{}
	tempShardStates := make(map[byte][]ShardState)
	stakeInstructions := [][]string{}
	validStakePublicKeys := []string{}
	swapInstructions := make(map[byte][][]string)
	bridgeInstructions := [][]string{}
	acceptedBlockRewardInstructions := [][]string{}
	stopAutoStakingInstructions := [][]string{}
	statefulActionsByShardID := map[byte][][]string{}
	rewardForCustodianByEpoch := map[common.Hash]uint64{}

	portalParams := blockchain.GetPortalParams(beaconBlock.GetHeight())

	// Get Reward Instruction By Epoch
	if beaconBlock.Header.Height%blockchain.config.ChainParams.Epoch == 1 {
		featureStateDB := curView.GetBeaconFeatureStateDB()
		totalLockedCollateral, err := getTotalLockedCollateralInEpoch(featureStateDB)
		if err != nil {
			return NewBlockChainError(GetTotalLockedCollateralError, err)
		}
		isSplitRewardForCustodian := totalLockedCollateral > 0
		percentCustodianRewards := portalParams.MaxPercentCustodianRewards
		if totalLockedCollateral < portalParams.MinLockCollateralAmountInEpoch {
			percentCustodianRewards = portalParams.MinPercentCustodianRewards
		}

		rewardByEpochInstruction, rewardForCustodianByEpoch, err = blockchain.buildRewardInstructionByEpoch(curView, beaconBlock.Header.Height, beaconBlock.Header.Epoch-1, curView.GetBeaconRewardStateDB(), isSplitRewardForCustodian, percentCustodianRewards)
		if err != nil {
			return NewBlockChainError(BuildRewardInstructionError, err)
		}
	}
	// get shard to beacon blocks from pool
	s2bRequired := make(map[byte][]common.Hash)
	var keys []int
	for k, shardstates := range beaconBlock.Body.ShardState {
		keys = append(keys, int(k))
		for _, state := range shardstates {
			s2bRequired[k] = append(s2bRequired[k], state.Hash)
		}
	}
	sort.Ints(keys)

	var allShardBlocks = make([][]*ShardToBeaconBlock, blockchain.config.ChainParams.ActiveShards)
	s2bBlocksFromPool, err := blockchain.config.Syncker.GetS2BBlocksForBeaconValidator(curView.BestShardHash, s2bRequired)
	if err != nil {
		return NewBlockChainError(GetShardToBeaconBlocksError, fmt.Errorf("Unable to get required s2bBlocks from pool in time"))
	}
	for sid, v := range s2bBlocksFromPool {
		for _, b := range v {
			allShardBlocks[sid] = append(allShardBlocks[sid], b.(*ShardToBeaconBlock))
		}
	}

	for chainID, shardBlocks := range allShardBlocks {
		shardID := byte(chainID)
		shardStates := beaconBlock.Body.ShardState[shardID]
		// repeatly compare each shard to beacon block and shard state in new beacon block body
		if len(shardBlocks) >= len(shardStates) {
			shardBlocks = shardBlocks[:len(beaconBlock.Body.ShardState[shardID])]

			for _, shardBlock := range shardBlocks {
				tempShardState, stakeInstruction, tempValidStakePublicKeys, swapInstruction, bridgeInstruction, acceptedBlockRewardInstruction, stopAutoStakingInstruction, statefulActions := blockchain.GetShardStateFromBlock(curView, beaconBlock.Header.Height, shardBlock, shardID, false, validStakePublicKeys)
				tempShardStates[shardID] = append(tempShardStates[shardID], tempShardState[shardID])
				stakeInstructions = append(stakeInstructions, stakeInstruction...)
				swapInstructions[shardID] = append(swapInstructions[shardID], swapInstruction[shardID]...)
				bridgeInstructions = append(bridgeInstructions, bridgeInstruction...)
				acceptedBlockRewardInstructions = append(acceptedBlockRewardInstructions, acceptedBlockRewardInstruction)
				stopAutoStakingInstructions = append(stopAutoStakingInstructions, stopAutoStakingInstruction...)
				validStakePublicKeys = append(validStakePublicKeys, tempValidStakePublicKeys...)
				// group stateful actions by shardID
				_, found := statefulActionsByShardID[shardID]
				if !found {
					statefulActionsByShardID[shardID] = statefulActions
				} else {
					statefulActionsByShardID[shardID] = append(statefulActionsByShardID[shardID], statefulActions...)
				}
			}
		} else {
			return NewBlockChainError(GetShardToBeaconBlocksError, fmt.Errorf("Expect to get more than %+v ShardToBeaconBlock but only get %+v (shard %v)", len(beaconBlock.Body.ShardState[shardID]), len(shardBlocks), shardID))
		}
	}
	// build stateful instructions
	statefulInsts := blockchain.buildStatefulInstructions(curView.featureStateDB, statefulActionsByShardID, beaconBlock.Header.Height, rewardForCustodianByEpoch, portalParams)
	bridgeInstructions = append(bridgeInstructions, statefulInsts...)

	tempInstruction, err := curView.GenerateInstruction(beaconBlock.Header.Height,
		stakeInstructions, swapInstructions, stopAutoStakingInstructions,
		curView.CandidateShardWaitingForCurrentRandom,
		bridgeInstructions, acceptedBlockRewardInstructions,
		blockchain.config.ChainParams.Epoch, blockchain.config.ChainParams.RandomTime, blockchain)
	if err != nil {
		return err
	}
	if len(rewardByEpochInstruction) != 0 {
		tempInstruction = append(tempInstruction, rewardByEpochInstruction...)
	}
	tempInstructionArr := []string{}
	for _, strs := range tempInstruction {
		tempInstructionArr = append(tempInstructionArr, strs...)
	}
	tempInstructionHash, err := generateHashFromStringArray(tempInstructionArr)
	if err != nil {
		return NewBlockChainError(GenerateInstructionHashError, fmt.Errorf("Fail to generate hash for instruction %+v", tempInstructionArr))
	}
	if !tempInstructionHash.IsEqual(&beaconBlock.Header.InstructionHash) {
		return NewBlockChainError(InstructionHashError, fmt.Errorf("Expect Instruction Hash in Beacon Header to be %+v, but get %+v, validator instructions: %+v", beaconBlock.Header.InstructionHash, tempInstructionHash, tempInstruction))
	}
	beaconVerifyPreprocesingForPreSignTimer.UpdateSince(startTimeVerifyPreProcessingBeaconBlockForSigning)
	return nil
}

//  verifyBestStateWithBeaconBlock verify the validation of a block with some best state in cache or current best state
//  Get beacon state of this block
//  For example, new blockHeight is 91 then beacon state of this block must have height 90
//  OR new block has previous has is beacon best block hash
//  - Get producer via index and compare with producer address in beacon block headerproce
//  - Validate public key and signature sanity
//  - Validate Agg Signature
//  - Beacon Best State has best block is previous block of new beacon block
//  - Beacon Best State has height compatible with new beacon block
//  - Beacon Best State has epoch compatible with new beacon block
//  - Beacon Best State has best shard height compatible with shard state of new beacon block
//  - New Stake public key must not found in beacon best state (candidate, pending validator, committee)
func (beaconBestState *BeaconBestState) verifyBestStateWithBeaconBlock(blockchain *BlockChain, beaconBlock *BeaconBlock, isVerifySig bool, chainParamEpoch uint64) error {
	//verify producer via index
	startTimeVerifyWithBestState := time.Now()
	if err := blockchain.config.ConsensusEngine.ValidateProducerPosition(beaconBlock, beaconBestState.BeaconProposerIndex, beaconBestState.BeaconCommittee); err != nil {
		return err
	}

	//=============End Verify Aggegrate signature
	if !beaconBestState.BestBlockHash.IsEqual(&beaconBlock.Header.PreviousBlockHash) {
		return NewBlockChainError(BeaconBestStateBestBlockNotCompatibleError, errors.New("previous us block should be :"+beaconBestState.BestBlockHash.String()))
	}
	if beaconBestState.BeaconHeight+1 != beaconBlock.Header.Height {
		return NewBlockChainError(WrongBlockHeightError, errors.New("block height of new block should be :"+strconv.Itoa(int(beaconBlock.Header.Height+1))))
	}
	if beaconBlock.Header.Height%chainParamEpoch == 1 && beaconBestState.Epoch+1 != beaconBlock.Header.Epoch {
		return NewBlockChainError(WrongEpochError, fmt.Errorf("Expect beacon block height %+v has epoch %+v but get %+v", beaconBlock.Header.Height, beaconBestState.Epoch+1, beaconBlock.Header.Epoch))
	}
	if beaconBlock.Header.Height%chainParamEpoch != 1 && beaconBestState.Epoch != beaconBlock.Header.Epoch {
		return NewBlockChainError(WrongEpochError, fmt.Errorf("Expect beacon block height %+v has epoch %+v but get %+v", beaconBlock.Header.Height, beaconBestState.Epoch, beaconBlock.Header.Epoch))
	}
	// check shard states of new beacon block and beacon best state
	// shard state of new beacon block must be greater or equal to current best shard height
	for shardID, shardStates := range beaconBlock.Body.ShardState {
		if bestShardHeight, ok := beaconBestState.BestShardHeight[shardID]; !ok {
			if shardStates[0].Height != 2 {
				return NewBlockChainError(BeaconBestStateBestShardHeightNotCompatibleError, fmt.Errorf("Shard %+v best height not found in beacon best state", shardID))
			}
		} else {
			if len(shardStates) > 0 {
				if bestShardHeight > shardStates[0].Height {
					return NewBlockChainError(BeaconBestStateBestShardHeightNotCompatibleError, fmt.Errorf("Expect Shard %+v has state greater than to %+v but get %+v", shardID, bestShardHeight, shardStates[0].Height))
				}
				if bestShardHeight < shardStates[0].Height && bestShardHeight+1 != shardStates[0].Height {
					return NewBlockChainError(BeaconBestStateBestShardHeightNotCompatibleError, fmt.Errorf("Expect Shard %+v has state %+v but get %+v", shardID, bestShardHeight+1, shardStates[0].Height))
				}
			}
		}
	}
	//=============Verify Stake Public Key
	newBeaconCandidate, newShardCandidate := GetStakingCandidate(*beaconBlock)
	if !reflect.DeepEqual(newBeaconCandidate, []string{}) {
		validBeaconCandidate := beaconBestState.GetValidStakers(newBeaconCandidate)
		if !reflect.DeepEqual(validBeaconCandidate, newBeaconCandidate) {
			return NewBlockChainError(CandidateError, errors.New("beacon candidate list is INVALID"))
		}
	}
	if !reflect.DeepEqual(newShardCandidate, []string{}) {
		validShardCandidate := beaconBestState.GetValidStakers(newShardCandidate)
		if !reflect.DeepEqual(validShardCandidate, newShardCandidate) {
			return NewBlockChainError(CandidateError, errors.New("shard candidate list is INVALID"))
		}
	}
	//=============End Verify Stakers
	beaconVerifyWithBestStateTimer.UpdateSince(startTimeVerifyWithBestState)
	return nil
}

//  verifyPostProcessingBeaconBlock verify block after update beacon best state
//  - Validator root: BeaconCommittee + BeaconPendingValidator
//  - Beacon Candidate root: CandidateBeaconWaitingForCurrentRandom + CandidateBeaconWaitingForNextRandom
//  - Shard Candidate root: CandidateShardWaitingForCurrentRandom + CandidateShardWaitingForNextRandom
//  - Shard Validator root: ShardCommittee + ShardPendingValidator
//  - Random number if have in instruction
func (beaconBestState *BeaconBestState) verifyPostProcessingBeaconBlock(beaconBlock *BeaconBlock, randomClient btc.RandomClient) error {
	var (
		strs []string
	)
	startTimeVerifyPostProcessingBeaconBlock := time.Now()
	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconCommittee)
	if err != nil {
		panic(err)
	}
	strs = append(strs, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconPendingValidator)
	if err != nil {
		panic(err)
	}
	strs = append(strs, beaconPendingValidatorStr...)
	if hash, ok := verifyHashFromStringArray(strs, beaconBlock.Header.BeaconCommitteeAndValidatorRoot); !ok {
		return NewBlockChainError(BeaconCommitteeAndPendingValidatorRootError, fmt.Errorf("Expect Beacon Committee and Validator Root to be %+v but get %+v", beaconBlock.Header.BeaconCommitteeAndValidatorRoot, hash))
	}
	strs = []string{}

	candidateBeaconWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	strs = append(strs, candidateBeaconWaitingForCurrentRandomStr...)

	candidateBeaconWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	strs = append(strs, candidateBeaconWaitingForNextRandomStr...)
	if hash, ok := verifyHashFromStringArray(strs, beaconBlock.Header.BeaconCandidateRoot); !ok {
		return NewBlockChainError(BeaconCandidateRootError, fmt.Errorf("Expect Beacon Committee and Validator Root to be %+v but get %+v", beaconBlock.Header.BeaconCandidateRoot, hash))
	}
	strs = []string{}

	candidateShardWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	strs = append(strs, candidateShardWaitingForCurrentRandomStr...)

	candidateShardWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	strs = append(strs, candidateShardWaitingForNextRandomStr...)
	if hash, ok := verifyHashFromStringArray(strs, beaconBlock.Header.ShardCandidateRoot); !ok {
		return NewBlockChainError(ShardCandidateRootError, fmt.Errorf("Expect Beacon Committee and Validator Root to be %+v but get %+v", beaconBlock.Header.ShardCandidateRoot, hash))
	}

	shardPendingValidator := make(map[byte][]string)
	for shardID, keyList := range beaconBestState.ShardPendingValidator {
		keyListStr, err := incognitokey.CommitteeKeyListToString(keyList)
		if err != nil {
			return err
		}
		shardPendingValidator[shardID] = keyListStr
	}

	shardCommittee := make(map[byte][]string)
	for shardID, keyList := range beaconBestState.ShardCommittee {
		keyListStr, err := incognitokey.CommitteeKeyListToString(keyList)
		if err != nil {
			return err
		}
		shardCommittee[shardID] = keyListStr
	}
	ok := verifyHashFromMapByteString(shardPendingValidator, shardCommittee, beaconBlock.Header.ShardCommitteeAndValidatorRoot)
	if !ok {
		return NewBlockChainError(ShardCommitteeAndPendingValidatorRootError, fmt.Errorf("Expect Beacon Committee and Validator Root to be %+v", beaconBlock.Header.ShardCommitteeAndValidatorRoot))
	}
	if hash, ok := verifyHashFromMapStringBool(beaconBestState.AutoStaking, beaconBlock.Header.AutoStakingRoot); !ok {
		return NewBlockChainError(ShardCommitteeAndPendingValidatorRootError, fmt.Errorf("Expect Beacon Committee and Validator Root to be %+v but get %+v", beaconBlock.Header.AutoStakingRoot, hash))
	}
	if !TestRandom {
		//COMMENT FOR TESTING
		instructions := beaconBlock.Body.Instructions
		for _, l := range instructions {
			if l[0] == "random" {
				startTime := time.Now()
				// ["random" "{nonce}" "{blockheight}" "{timestamp}" "{bitcoinTimestamp}"]
				nonce, err := strconv.Atoi(l[1])
				if err != nil {
					Logger.log.Errorf("Blockchain Error %+v", NewBlockChainError(UnExpectedError, err))
					return NewBlockChainError(UnExpectedError, err)
				}
				ok, err = randomClient.VerifyNonceWithTimestamp(startTime, beaconBestState.BlockMaxCreateTime, beaconBestState.CurrentRandomTimeStamp, int64(nonce))
				Logger.log.Infof("Verify Random number %+v", ok)
				if err != nil {
					Logger.log.Error("Blockchain Error %+v", NewBlockChainError(UnExpectedError, err))
					return NewBlockChainError(UnExpectedError, err)
				}
				if !ok {
					return NewBlockChainError(RandomError, errors.New("Error verify random number"))
				}
			}
		}
	}
	beaconVerifyPostProcessingTimer.UpdateSince(startTimeVerifyPostProcessingBeaconBlock)
	return nil
}

/*
	Update Beststate with new Block
*/
func (oldBestState *BeaconBestState) updateBeaconBestState(beaconBlock *BeaconBlock, chainParamEpoch uint64, chainParamAssignOffset int, randomTime uint64, committeeChange *committeeChange) (*BeaconBestState, error) {
	startTimeUpdateBeaconBestState := time.Now()
	beaconBestState := NewBeaconBestState()
	if err := beaconBestState.cloneBeaconBestStateFrom(oldBestState); err != nil {
		return nil, err
	}

	Logger.log.Debugf("Start processing new block at height %d, with hash %+v", beaconBlock.Header.Height, *beaconBlock.Hash())
	newBeaconCandidate := []incognitokey.CommitteePublicKey{}
	newShardCandidate := []incognitokey.CommitteePublicKey{}
	// Logger.log.Infof("Start processing new block at height %d, with hash %+v", newBlock.Header.Height, *newBlock.Hash())
	if beaconBlock == nil {
		return nil, errors.New("null pointer")
	}
	// signal of random parameter from beacon block
	randomFlag := false
	// update BestShardHash, BestBlock, BestBlockHash
	beaconBestState.PreviousBestBlockHash = beaconBestState.BestBlockHash
	beaconBestState.BestBlockHash = *beaconBlock.Hash()
	beaconBestState.BestBlock = *beaconBlock
	beaconBestState.Epoch = beaconBlock.Header.Epoch
	beaconBestState.BeaconHeight = beaconBlock.Header.Height
	if beaconBlock.Header.Height == 1 {
		beaconBestState.BeaconProposerIndex = 0
	} else {
		for i, v := range oldBestState.BeaconCommittee {
			b58Str, _ := v.ToBase58()
			if b58Str == beaconBlock.Header.Producer {
				beaconBestState.BeaconProposerIndex = i
				break
			}
		}
	}
	if beaconBestState.BestShardHash == nil {
		beaconBestState.BestShardHash = make(map[byte]common.Hash)
	}
	if beaconBestState.BestShardHeight == nil {
		beaconBestState.BestShardHeight = make(map[byte]uint64)
	}
	// Update new best new block hash
	for shardID, shardStates := range beaconBlock.Body.ShardState {
		beaconBestState.BestShardHash[shardID] = shardStates[len(shardStates)-1].Hash
		beaconBestState.BestShardHeight[shardID] = shardStates[len(shardStates)-1].Height
	}
	// processing instruction
	snapshotAutoStaking := make(map[string]bool)
	for k, v := range beaconBestState.AutoStaking {
		snapshotAutoStaking[k] = v
	}
	for _, instruction := range beaconBlock.Body.Instructions {
		err, tempRandomFlag, tempNewBeaconCandidate, tempNewShardCandidate := beaconBestState.processInstruction(instruction, committeeChange, snapshotAutoStaking)
		if err != nil {
			return nil, err
		}
		if tempRandomFlag {
			randomFlag = tempRandomFlag
		}
		if len(tempNewBeaconCandidate) > 0 {
			newBeaconCandidate = append(newBeaconCandidate, tempNewBeaconCandidate...)
		}
		if len(tempNewShardCandidate) > 0 {
			newShardCandidate = append(newShardCandidate, tempNewShardCandidate...)
		}
	}
	// update candidate list after processing instructions
	beaconBestState.CandidateBeaconWaitingForNextRandom = append(beaconBestState.CandidateBeaconWaitingForNextRandom, newBeaconCandidate...)
	committeeChange.nextEpochBeaconCandidateAdded = append(committeeChange.nextEpochBeaconCandidateAdded, newBeaconCandidate...)
	beaconBestState.CandidateShardWaitingForNextRandom = append(beaconBestState.CandidateShardWaitingForNextRandom, newShardCandidate...)
	committeeChange.nextEpochShardCandidateAdded = append(committeeChange.nextEpochShardCandidateAdded, newShardCandidate...)
	if beaconBestState.BeaconHeight%chainParamEpoch == 1 && beaconBestState.BeaconHeight != 1 {
		// Begin of each epoch
		beaconBestState.IsGetRandomNumber = false
		// Before get random from bitcoin
	} else if beaconBestState.BeaconHeight%chainParamEpoch >= randomTime {
		// snap shot candidate list, prepare to get random number (beaconHeight == random time)
		if beaconBestState.BeaconHeight%chainParamEpoch == randomTime {
			// snapshot candidate list
			committeeChange.currentEpochShardCandidateAdded = beaconBestState.CandidateShardWaitingForNextRandom
			beaconBestState.CandidateShardWaitingForCurrentRandom = beaconBestState.CandidateShardWaitingForNextRandom
			committeeChange.currentEpochBeaconCandidateAdded = beaconBestState.CandidateBeaconWaitingForNextRandom
			beaconBestState.CandidateBeaconWaitingForCurrentRandom = beaconBestState.CandidateBeaconWaitingForNextRandom
			Logger.log.Debug("Beacon Process: CandidateShardWaitingForCurrentRandom: ", beaconBestState.CandidateShardWaitingForCurrentRandom)
			Logger.log.Debug("Beacon Process: CandidateBeaconWaitingForCurrentRandom: ", beaconBestState.CandidateBeaconWaitingForCurrentRandom)
			// reset candidate list
			committeeChange.nextEpochShardCandidateRemoved = beaconBestState.CandidateShardWaitingForNextRandom
			beaconBestState.CandidateShardWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
			committeeChange.nextEpochBeaconCandidateRemoved = beaconBestState.CandidateBeaconWaitingForNextRandom
			beaconBestState.CandidateBeaconWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
			// assign random timestamp
			beaconBestState.CurrentRandomTimeStamp = beaconBlock.Header.Timestamp
		}
		// if get new random number (beaconHeight > random time)
		// Assign candidate to shard
		// assign CandidateShardWaitingForCurrentRandom to ShardPendingValidator with CurrentRandom
		if randomFlag {
			beaconBestState.IsGetRandomNumber = true
			numberOfPendingValidator := make(map[byte]int)
			for shardID, pendingValidators := range beaconBestState.ShardPendingValidator {
				numberOfPendingValidator[shardID] = len(pendingValidators)
			}
			shardCandidatesStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForCurrentRandom)
			if err != nil {
				panic(err)
			}
			remainShardCandidatesStr, assignedCandidates := assignShardCandidate(shardCandidatesStr, numberOfPendingValidator, beaconBestState.CurrentRandomNumber, chainParamAssignOffset, beaconBestState.ActiveShards)
			remainShardCandidates, err := incognitokey.CommitteeBase58KeyListToStruct(remainShardCandidatesStr)
			if err != nil {
				panic(err)
			}
			committeeChange.nextEpochShardCandidateAdded = append(committeeChange.nextEpochShardCandidateAdded, remainShardCandidates...)
			// append remain candidate into shard waiting for next random list
			beaconBestState.CandidateShardWaitingForNextRandom = append(beaconBestState.CandidateShardWaitingForNextRandom, remainShardCandidates...)
			// assign candidate into shard pending validator list
			for shardID, candidateListStr := range assignedCandidates {
				candidateList, err := incognitokey.CommitteeBase58KeyListToStruct(candidateListStr)
				if err != nil {
					panic(err)
				}
				committeeChange.shardSubstituteAdded[shardID] = candidateList
				beaconBestState.ShardPendingValidator[shardID] = append(beaconBestState.ShardPendingValidator[shardID], candidateList...)
			}
			committeeChange.currentEpochShardCandidateRemoved = beaconBestState.CandidateShardWaitingForCurrentRandom
			// delete CandidateShardWaitingForCurrentRandom list
			beaconBestState.CandidateShardWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
			// shuffle CandidateBeaconWaitingForCurrentRandom with current random number
			newBeaconPendingValidator, err := ShuffleCandidate(beaconBestState.CandidateBeaconWaitingForCurrentRandom, beaconBestState.CurrentRandomNumber)
			if err != nil {
				return nil, NewBlockChainError(ShuffleBeaconCandidateError, err)
			}
			committeeChange.currentEpochBeaconCandidateRemoved = beaconBestState.CandidateBeaconWaitingForCurrentRandom
			beaconBestState.CandidateBeaconWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
			committeeChange.beaconSubstituteAdded = newBeaconPendingValidator
			beaconBestState.BeaconPendingValidator = append(beaconBestState.BeaconPendingValidator, newBeaconPendingValidator...)
		}
	}
	if err := beaconBestState.processAutoStakingChange(committeeChange); err != nil {
		return nil, NewBlockChainError(ProcessAutoStakingError, err)
	}
	beaconBestState.updateNumOfBlocksByProducers(beaconBlock, chainParamEpoch)
	beaconUpdateBestStateTimer.UpdateSince(startTimeUpdateBeaconBestState)
	return beaconBestState, nil
}

func (beaconBestState *BeaconBestState) initBeaconBestState(genesisBeaconBlock *BeaconBlock, db incdb.Database) error {
	var (
		newBeaconCandidate = []incognitokey.CommitteePublicKey{}
		newShardCandidate  = []incognitokey.CommitteePublicKey{}
	)
	Logger.log.Info("Process Update Beacon Best State With Beacon Genesis Block")
	beaconBestState.PreviousBestBlockHash = beaconBestState.BestBlockHash
	beaconBestState.BestBlockHash = *genesisBeaconBlock.Hash()
	beaconBestState.BestBlock = *genesisBeaconBlock
	beaconBestState.Epoch = genesisBeaconBlock.Header.Epoch
	beaconBestState.BeaconHeight = genesisBeaconBlock.Header.Height
	beaconBestState.BeaconProposerIndex = 0
	beaconBestState.BestShardHash = make(map[byte]common.Hash)
	beaconBestState.BestShardHeight = make(map[byte]uint64)
	// Update new best new block hash
	for shardID, shardStates := range genesisBeaconBlock.Body.ShardState {
		beaconBestState.BestShardHash[shardID] = shardStates[len(shardStates)-1].Hash
		beaconBestState.BestShardHeight[shardID] = shardStates[len(shardStates)-1].Height
	}
	// update param
	snapshotAutoStaking := make(map[string]bool)
	for k, v := range beaconBestState.AutoStaking {
		snapshotAutoStaking[k] = v
	}
	for _, instruction := range genesisBeaconBlock.Body.Instructions {
		err, _, tempNewBeaconCandidate, tempNewShardCandidate := beaconBestState.processInstruction(instruction, newCommitteeChange(), snapshotAutoStaking)
		if err != nil {
			return err
		}
		newBeaconCandidate = append(newBeaconCandidate, tempNewBeaconCandidate...)
		newShardCandidate = append(newShardCandidate, tempNewShardCandidate...)
	}
	beaconBestState.BeaconCommittee = append(beaconBestState.BeaconCommittee, newBeaconCandidate...)
	beaconBestState.ConsensusAlgorithm = common.BlsConsensus
	beaconBestState.ShardConsensusAlgorithm = make(map[byte]string)
	for shardID := 0; shardID < beaconBestState.ActiveShards; shardID++ {
		beaconBestState.ShardCommittee[byte(shardID)] = append(beaconBestState.ShardCommittee[byte(shardID)], newShardCandidate[shardID*beaconBestState.MinShardCommitteeSize:(shardID+1)*beaconBestState.MinShardCommitteeSize]...)
		beaconBestState.ShardConsensusAlgorithm[byte(shardID)] = common.BlsConsensus
	}
	beaconBestState.Epoch = 1
	beaconBestState.NumOfBlocksByProducers = make(map[string]uint64)
	//statedb===========================START
	var err error
	dbAccessWarper := statedb.NewDatabaseAccessWarper(db)
	beaconBestState.featureStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.consensusStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.rewardStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.slashStateDB, err = statedb.NewWithPrefixTrie(common.EmptyRoot, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.ConsensusStateDBRootHash = common.EmptyRoot
	beaconBestState.SlashStateDBRootHash = common.EmptyRoot
	beaconBestState.RewardStateDBRootHash = common.EmptyRoot
	beaconBestState.FeatureStateDBRootHash = common.EmptyRoot
	beaconBestState.currentPDEState, err = InitCurrentPDEStateFromDB(beaconBestState.featureStateDB, beaconBestState.BeaconHeight)
	if err != nil {
		return err
	}
	//statedb===========================END
	return nil
}

//  processInstruction, process these instruction:
//  - Random Instruction format
//		["random" "{nonce}" "{blockheight}" "{timestamp}" "{bitcoinTimestamp}"]
//	- store random number into beststate
//  - Swap Instruction format
//		["swap" "inPubkey1,inPubkey2,..." "outPupkey1, outPubkey2,..." "shard" "shardID"]
//		["swap" "inPubkey1,inPubkey2,..." "outPupkey1, outPubkey2,..." "beacon"]
//    + Update shard/beacon pending validator and shard/beacon committee in beststate
//  - Stake Instruction
//	  + format
//		["stake", "pubkey1,pubkey2,..." "shard" "txStake1,txStake2,..." "rewardReceiver1,rewardReceiver2,..." flag]
//		["stake", "pubkey1,pubkey2,..." "beacon" "txStake1,txStake2,..." "rewardReceiver1,rewardReceiver2,..." flag]
//	  + Get Stake public key and for later storage
//  Return param
//  #1 error
//  #2 random flag
//  #3 new beacon candidate
//  #4 new shard candidate

func (beaconBestState *BeaconBestState) processInstruction(instruction []string, committeeChange *committeeChange, autoStaking map[string]bool) (error, bool, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey) {
	newBeaconCandidates := []incognitokey.CommitteePublicKey{}
	newShardCandidates := []incognitokey.CommitteePublicKey{}
	if len(instruction) < 1 {
		return nil, false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
	}
	// ["random" "{nonce}" "{blockheight}" "{timestamp}" "{bitcoinTimestamp}"]
	if instruction[0] == RandomAction {
		temp, err := strconv.Atoi(instruction[1])
		if err != nil {
			return NewBlockChainError(ProcessRandomInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		beaconBestState.CurrentRandomNumber = int64(temp)
		Logger.log.Infof("Random number found %d", beaconBestState.CurrentRandomNumber)
		return nil, true, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
	}
	if instruction[0] == StopAutoStake {
		committeePublicKeys := strings.Split(instruction[1], ",")
		for _, committeePublicKey := range committeePublicKeys {
			allCommitteeValidatorCandidate := beaconBestState.getAllCommitteeValidatorCandidateFlattenList()
			// check existence in all committee list
			if common.IndexOfStr(committeePublicKey, allCommitteeValidatorCandidate) == -1 {
				// if not found then delete auto staking data for this public key if present
				if _, ok := beaconBestState.AutoStaking[committeePublicKey]; ok {
					delete(beaconBestState.AutoStaking, committeePublicKey)
				}
			} else {
				// if found in committee list then turn off auto staking
				if _, ok := beaconBestState.AutoStaking[committeePublicKey]; ok {
					beaconBestState.AutoStaking[committeePublicKey] = false
					committeeChange.stopAutoStaking = append(committeeChange.stopAutoStaking, committeePublicKey)
				}
			}
		}
	}
	if instruction[0] == SwapAction {
		Logger.log.Debug("Swap Instruction", instruction)
		inPublickeys := strings.Split(instruction[1], ",")
		Logger.log.Debug("Swap Instruction In Public Keys", inPublickeys)
		inPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(inPublickeys)
		if err != nil {
			return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		outPublickeys := strings.Split(instruction[2], ",")
		Logger.log.Debug("Swap Instruction Out Public Keys", outPublickeys)
		outPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(outPublickeys)
		if err != nil {
			if len(outPublickeys) != 0 {
				return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
			}
		}

		if instruction[3] == "shard" {
			temp, err := strconv.Atoi(instruction[4])
			if err != nil {
				return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
			}
			shardID := byte(temp)
			// delete in public key out of sharding pending validator list
			if len(instruction[1]) > 0 {
				shardPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.ShardPendingValidator[shardID])
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				tempShardPendingValidator, err := RemoveValidator(shardPendingValidatorStr, inPublickeys)
				if err != nil {
					return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// update shard pending validator
				committeeChange.shardSubstituteRemoved[shardID] = append(committeeChange.shardSubstituteRemoved[shardID], inPublickeyStructs...)
				beaconBestState.ShardPendingValidator[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardPendingValidator)
				if err != nil {
					return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// add new public key to committees
				committeeChange.shardCommitteeAdded[shardID] = append(committeeChange.shardCommitteeAdded[shardID], inPublickeyStructs...)
				beaconBestState.ShardCommittee[shardID] = append(beaconBestState.ShardCommittee[shardID], inPublickeyStructs...)
			}
			// delete out public key out of current committees
			if len(instruction[2]) > 0 {
				//for _, value := range outPublickeyStructs {
				//	delete(beaconBestState.RewardReceiver, value.GetIncKeyBase58())
				//}
				shardCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.ShardCommittee[shardID])
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				tempShardCommittees, err := RemoveValidator(shardCommitteeStr, outPublickeys)
				if err != nil {
					return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// remove old public key in shard committee update shard committee
				committeeChange.shardCommitteeRemoved[shardID] = append(committeeChange.shardCommitteeRemoved[shardID], outPublickeyStructs...)
				beaconBestState.ShardCommittee[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardCommittees)
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// Check auto stake in out public keys list
				// if auto staking not found or flag auto stake is false then do not re-stake for this out public key
				// if auto staking flag is true then system will automatically add this out public key to current candidate list
				for index, outPublicKey := range outPublickeys {
					if isAutoStaking, ok := autoStaking[outPublicKey]; !ok {
						if _, ok := beaconBestState.RewardReceiver[outPublicKey]; ok {
							delete(beaconBestState.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoStaking {
							// delete this flag for next time staking
							delete(beaconBestState.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
							delete(beaconBestState.AutoStaking, outPublicKey)
						} else {
							shardCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
							}
							newShardCandidates = append(newShardCandidates, shardCandidate...)
						}
					}
				}
			}
		} else if instruction[3] == "beacon" {
			if len(instruction[1]) > 0 {
				beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconPendingValidator)
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				tempBeaconPendingValidator, err := RemoveValidator(beaconPendingValidatorStr, inPublickeys)
				if err != nil {
					return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// update beacon pending validator
				committeeChange.beaconSubstituteRemoved = append(committeeChange.beaconSubstituteRemoved, inPublickeyStructs...)
				beaconBestState.BeaconPendingValidator, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconPendingValidator)
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// add new public key to beacon committee
				committeeChange.beaconCommitteeAdded = append(committeeChange.beaconCommitteeAdded, inPublickeyStructs...)
				beaconBestState.BeaconCommittee = append(beaconBestState.BeaconCommittee, inPublickeyStructs...)
			}
			if len(instruction[2]) > 0 {
				// delete reward receiver
				//for _, value := range outPublickeyStructs {
				//	delete(beaconBestState.RewardReceiver, value.GetIncKeyBase58())
				//}
				beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconCommittee)
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				tempBeaconCommittes, err := RemoveValidator(beaconCommitteeStr, outPublickeys)
				if err != nil {
					return NewBlockChainError(ProcessSwapInstructionError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				// remove old public key in beacon committee and update beacon best state
				committeeChange.beaconCommitteeRemoved = append(committeeChange.beaconCommitteeRemoved, outPublickeyStructs...)
				beaconBestState.BeaconCommittee, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconCommittes)
				if err != nil {
					return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
				}
				for index, outPublicKey := range outPublickeys {
					if isAutoStaking, ok := autoStaking[outPublicKey]; !ok {
						if _, ok := beaconBestState.RewardReceiver[outPublicKey]; ok {
							delete(beaconBestState.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoStaking {
							delete(beaconBestState.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
							delete(beaconBestState.AutoStaking, outPublicKey)
						} else {
							beaconCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
							}
							newBeaconCandidates = append(newBeaconCandidates, beaconCandidate...)
						}
					}
				}
			}
		}
		return nil, false, newBeaconCandidates, newShardCandidates
	}
	// Update candidate
	// get staking candidate list and store
	// store new staking candidate
	if instruction[0] == StakeAction && instruction[2] == "beacon" {
		beaconCandidates := strings.Split(instruction[1], ",")
		beaconCandidatesStructs, err := incognitokey.CommitteeBase58KeyListToStruct(beaconCandidates)
		if err != nil {
			return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		beaconRewardReceivers := strings.Split(instruction[4], ",")
		beaconAutoReStaking := strings.Split(instruction[5], ",")
		if len(beaconCandidatesStructs) != len(beaconRewardReceivers) && len(beaconRewardReceivers) != len(beaconAutoReStaking) {
			return NewBlockChainError(StakeInstructionError, fmt.Errorf("Expect Beacon Candidate (length %+v) and Beacon Reward Receiver (length %+v) and Beacon Auto ReStaking (lenght %+v) have equal length", len(beaconCandidates), len(beaconRewardReceivers), len(beaconAutoReStaking))), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		for index, candidate := range beaconCandidatesStructs {
			beaconBestState.RewardReceiver[candidate.GetIncKeyBase58()] = beaconRewardReceivers[index]
			if beaconAutoReStaking[index] == "true" {
				beaconBestState.AutoStaking[beaconCandidates[index]] = true
			} else {
				beaconBestState.AutoStaking[beaconCandidates[index]] = false
			}
		}

		newBeaconCandidates = append(newBeaconCandidates, beaconCandidatesStructs...)
		return nil, false, newBeaconCandidates, newShardCandidates
	}
	if instruction[0] == StakeAction && instruction[2] == "shard" {
		shardCandidates := strings.Split(instruction[1], ",")
		shardCandidatesStructs, err := incognitokey.CommitteeBase58KeyListToStruct(shardCandidates)
		if err != nil {
			return NewBlockChainError(UnExpectedError, err), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		shardRewardReceivers := strings.Split(instruction[4], ",")
		shardAutoReStaking := strings.Split(instruction[5], ",")
		if len(shardCandidates) != len(shardRewardReceivers) && len(shardRewardReceivers) != len(shardAutoReStaking) {
			return NewBlockChainError(StakeInstructionError, fmt.Errorf("Expect Beacon Candidate (length %+v) and Beacon Reward Receiver (length %+v) and Shard Auto ReStaking (length %+v) have equal length", len(shardCandidates), len(shardRewardReceivers), len(shardAutoReStaking))), false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		}
		for index, candidate := range shardCandidatesStructs {
			beaconBestState.RewardReceiver[candidate.GetIncKeyBase58()] = shardRewardReceivers[index]
			if shardAutoReStaking[index] == "true" {
				beaconBestState.AutoStaking[shardCandidates[index]] = true
			} else {
				beaconBestState.AutoStaking[shardCandidates[index]] = false
			}
		}
		newShardCandidates = append(newShardCandidates, shardCandidatesStructs...)
		return nil, false, newBeaconCandidates, newShardCandidates
	}
	return nil, false, []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
}

func (beaconBestState *BeaconBestState) processAutoStakingChange(committeeChange *committeeChange) error {
	stopAutoStakingIncognitoKey, err := incognitokey.CommitteeBase58KeyListToStruct(committeeChange.stopAutoStaking)
	if err != nil {
		return err
	}
	for _, committeePublicKey := range stopAutoStakingIncognitoKey {
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.nextEpochBeaconCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.currentEpochBeaconCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.nextEpochShardCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.currentEpochShardCandidateAdded) > -1 {
			continue
		}
		flag := false
		for _, v := range committeeChange.shardSubstituteAdded {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		for _, v := range committeeChange.shardCommitteeAdded {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.beaconSubstituteAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.beaconCommitteeAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.CandidateBeaconWaitingForNextRandom) > -1 {
			committeeChange.nextEpochBeaconCandidateAdded = append(committeeChange.nextEpochBeaconCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.CandidateBeaconWaitingForCurrentRandom) > -1 {
			committeeChange.currentEpochBeaconCandidateAdded = append(committeeChange.currentEpochBeaconCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.CandidateShardWaitingForNextRandom) > -1 {
			committeeChange.nextEpochShardCandidateAdded = append(committeeChange.nextEpochShardCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.CandidateShardWaitingForCurrentRandom) > -1 {
			committeeChange.currentEpochShardCandidateAdded = append(committeeChange.currentEpochShardCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.BeaconPendingValidator) > -1 {
			committeeChange.beaconSubstituteAdded = append(committeeChange.beaconSubstituteAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, beaconBestState.BeaconCommittee) > -1 {
			committeeChange.beaconCommitteeAdded = append(committeeChange.beaconCommitteeAdded, committeePublicKey)
		}
		for k, v := range beaconBestState.ShardCommittee {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				committeeChange.shardCommitteeAdded[k] = append(committeeChange.shardCommitteeAdded[k], committeePublicKey)
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		for k, v := range beaconBestState.ShardPendingValidator {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				committeeChange.shardSubstituteAdded[k] = append(committeeChange.shardSubstituteAdded[k], committeePublicKey)
				flag = true
				break
			}
		}
		if flag {
			continue
		}
	}
	return nil
}

func (blockchain *BlockChain) processStoreBeaconBlock(
	newBestState *BeaconBestState,
	beaconBlock *BeaconBlock,
	snapshotBeaconCommittees []incognitokey.CommitteePublicKey,
	snapshotAllShardCommittees map[byte][]incognitokey.CommitteePublicKey,
	snapshotRewardReceivers map[string]string,
	committeeChange *committeeChange,
) error {
	startTimeProcessStoreBeaconBlock := time.Now()
	Logger.log.Debugf("BEACON | Process Store Beacon Block Height %+v with hash %+v", beaconBlock.Header.Height, beaconBlock.Header.Hash())
	blockHash := beaconBlock.Header.Hash()
	blockHeight := beaconBlock.Header.Height

	var err error
	//statedb===========================START
	// Added
	err = statedb.StoreCurrentEpochShardCandidate(newBestState.consensusStateDB, committeeChange.currentEpochShardCandidateAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreNextEpochShardCandidate(newBestState.consensusStateDB, committeeChange.nextEpochShardCandidateAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreCurrentEpochBeaconCandidate(newBestState.consensusStateDB, committeeChange.currentEpochBeaconCandidateAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreNextEpochBeaconCandidate(newBestState.consensusStateDB, committeeChange.nextEpochBeaconCandidateAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreAllShardSubstitutesValidator(newBestState.consensusStateDB, committeeChange.shardSubstituteAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreAllShardCommittee(newBestState.consensusStateDB, committeeChange.shardCommitteeAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreBeaconSubstituteValidator(newBestState.consensusStateDB, committeeChange.beaconSubstituteAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	err = statedb.StoreBeaconCommittee(newBestState.consensusStateDB, committeeChange.beaconCommitteeAdded, newBestState.RewardReceiver, newBestState.AutoStaking)
	if err != nil {
		return err
	}
	// Deleted
	err = statedb.DeleteCurrentEpochShardCandidate(newBestState.consensusStateDB, committeeChange.currentEpochShardCandidateRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteNextEpochShardCandidate(newBestState.consensusStateDB, committeeChange.nextEpochShardCandidateRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteCurrentEpochBeaconCandidate(newBestState.consensusStateDB, committeeChange.currentEpochBeaconCandidateRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteNextEpochBeaconCandidate(newBestState.consensusStateDB, committeeChange.nextEpochBeaconCandidateRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteAllShardSubstitutesValidator(newBestState.consensusStateDB, committeeChange.shardSubstituteRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteAllShardCommittee(newBestState.consensusStateDB, committeeChange.shardCommitteeRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteBeaconSubstituteValidator(newBestState.consensusStateDB, committeeChange.beaconSubstituteRemoved)
	if err != nil {
		return err
	}
	err = statedb.DeleteBeaconCommittee(newBestState.consensusStateDB, committeeChange.beaconCommitteeRemoved)
	if err != nil {
		return err
	}

	blockchain.processForSlashing(newBestState.slashStateDB, beaconBlock)

	// Remove shard reward request of old epoch
	// this value is no longer needed because, old epoch reward has been split and send to shard
	if beaconBlock.Header.Height%blockchain.config.ChainParams.Epoch == 2 {
		statedb.RemoveRewardOfShardByEpoch(newBestState.rewardStateDB, beaconBlock.Header.Epoch-1)
	}
	err = blockchain.addShardRewardRequestToBeacon(beaconBlock, newBestState.rewardStateDB)
	if err != nil {
		return NewBlockChainError(UpdateDatabaseWithBlockRewardInfoError, err)
	}
	// execute, store
	err = blockchain.processBridgeInstructions(newBestState.featureStateDB, beaconBlock)
	if err != nil {
		return NewBlockChainError(ProcessBridgeInstructionError, err)
	}
	// execute, store PDE instruction
	err = blockchain.processPDEInstructions(newBestState.featureStateDB, beaconBlock)
	if err != nil {
		return NewBlockChainError(ProcessPDEInstructionError, err)
	}

	// execute, store Portal Instruction
	if (blockchain.config.ChainParams.Net == Mainnet) || (blockchain.config.ChainParams.Net == Testnet && beaconBlock.Header.Height > 1500000) {
		err = blockchain.processPortalInstructions(newBestState.featureStateDB, beaconBlock)
		if err != nil {
			return NewBlockChainError(ProcessPortalInstructionError, err)
		}
	}

	// execute, store Ralaying Instruction
	err = blockchain.processRelayingInstructions(beaconBlock)
	if err != nil {
		return NewBlockChainError(ProcessPortalRelayingError, err)
	}

	//store beacon block hash by index to consensus state db => mark this block hash is for this view at this height
	if err := statedb.StoreBeaconBlockHashByIndex(newBestState.consensusStateDB, blockHeight, blockHash); err != nil {
		return err
	}

	consensusRootHash, err := newBestState.consensusStateDB.Commit(true)
	if err != nil {
		return err
	}

	err = newBestState.consensusStateDB.Database().TrieDB().Commit(consensusRootHash, false)
	if err != nil {
		return err
	}

	newBestState.ConsensusStateDBRootHash = consensusRootHash
	featureRootHash, err := newBestState.featureStateDB.Commit(true)
	if err != nil {
		return err
	}
	err = newBestState.featureStateDB.Database().TrieDB().Commit(featureRootHash, false)
	if err != nil {
		return err
	}
	newBestState.FeatureStateDBRootHash = featureRootHash
	rewardRootHash, err := newBestState.rewardStateDB.Commit(true)
	if err != nil {
		return err
	}
	err = newBestState.rewardStateDB.Database().TrieDB().Commit(rewardRootHash, false)
	if err != nil {
		return err
	}
	newBestState.RewardStateDBRootHash = rewardRootHash
	slashRootHash, err := newBestState.slashStateDB.Commit(true)
	if err != nil {
		return err
	}
	err = newBestState.slashStateDB.Database().TrieDB().Commit(slashRootHash, false)
	if err != nil {
		return err
	}
	newBestState.SlashStateDBRootHash = slashRootHash
	newBestState.consensusStateDB.ClearObjects()
	newBestState.rewardStateDB.ClearObjects()
	newBestState.featureStateDB.ClearObjects()
	newBestState.slashStateDB.ClearObjects()
	//statedb===========================END
	batch := blockchain.GetBeaconChainDatabase()
	//State Root Hash
	if err := rawdbv2.StoreBeaconConsensusStateRootHash(batch, blockHeight, consensusRootHash); err != nil {
		return NewBlockChainError(StoreBeaconBlockError, err)
	}
	if err := rawdbv2.StoreBeaconRewardStateRootHash(batch, blockHeight, rewardRootHash); err != nil {
		return NewBlockChainError(StoreBeaconBlockError, err)
	}
	if err := rawdbv2.StoreBeaconFeatureStateRootHash(batch, blockHeight, featureRootHash); err != nil {
		return NewBlockChainError(StoreBeaconBlockError, err)
	}
	if err := rawdbv2.StoreBeaconSlashStateRootHash(batch, blockHeight, slashRootHash); err != nil {
		return NewBlockChainError(StoreBeaconBlockError, err)
	}

	if err := rawdbv2.StoreBeaconBlock(batch, blockHeight, blockHash, beaconBlock); err != nil {
		return NewBlockChainError(StoreBeaconBlockError, err)
	}

	//fmt.Printf("debug AddView %s %+v\n", newBestState.Hash().String(), newBestState.BestBlock)
	//finalView := blockchain.BeaconChain.GetFinalView()
	blockchain.BeaconChain.multiView.AddView(newBestState)

	//newFinalView := blockchain.BeaconChain.GetFinalView()
	//if finalView != nil && newFinalView.GetHash().String() != finalView.GetHash().String() {
	//	startView := newFinalView
	//	for {
	//		hashs, _ := blockchain.GetBeaconBlockHashByHeight(startView.GetHeight())
	//		for _, hash := range hashs {
	//			if hash.String() != startView.GetHash().String() {
	//				err := rawdbv2.DeleteBeaconBlock(blockchain.GetBeaconChainDatabase(), startView.GetHeight(), hash)
	//				if err != nil {
	//					//must not have error
	//					panic(err)
	//				}
	//			}
	//		}
	//		startView = blockchain.BeaconChain.GetViewByHash(*startView.GetPreviousHash())
	//		if startView == nil || startView.GetHeight() == finalView.GetHeight() {
	//			break
	//		}
	//	}
	//}

	err = blockchain.BackupBeaconViews(batch)
	if err != nil {
		panic("Backup shard view error")
	}
	//if err := batch.Write(); err != nil {
	//	return NewBlockChainError(StoreBeaconBlockError, err)
	//}
	beaconStoreBlockTimer.UpdateSince(startTimeProcessStoreBeaconBlock)
	return nil
}
