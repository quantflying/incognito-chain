package block

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"time"
)

type BeaconCoreApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconCoreApp) preCreateBlock() error {
	state := s.CreateState
	curView := state.curView

	//TODO: get s2b blocks from pool => s2bBlks
	// -> Only accept block in one epoch
	state.s2bBlks = make(map[byte][]*ShardToBeaconBlock)

	//newEpoch? endEpoch? finalBlockInEpoch?
	if curView.GetHeight()+1%curView.BC.GetChainParams().Epoch == 1 {
		state.isNewEpoch = true
	}
	if curView.GetHeight()+1%curView.BC.GetChainParams().Epoch == 0 {
		state.isEndEpoch = true
	}
	if curView.GetHeight()%curView.BC.GetChainParams().Epoch == curView.BC.GetChainParams().Epoch-1 {
		state.isFinalBlockInEpoch = true
	}

	//shardstates
	shardStates := make(map[byte][]ShardState)
	for shardID, shardBlocks := range state.s2bBlks {
		for _, s2bBlk := range shardBlocks {
			shardState := ShardState{}
			shardState.CrossShard = make([]byte, len(s2bBlk.Header.CrossShardBitMap))
			copy(shardState.CrossShard, s2bBlk.Header.CrossShardBitMap)
			shardState.Hash = s2bBlk.Header.Hash()
			shardState.Height = s2bBlk.Header.Height
			shardStates[shardID] = append(shardStates[shardID], shardState)
		}
	}
	s.CreateState.shardStates = shardStates
	return nil
}

func (s *BeaconCoreApp) buildInstructionByEpoch() error {
	state := s.CreateState
	curView := state.curView

	//build reward instruction
	if state.isNewEpoch {
		var err error
		if state.rewardInstByEpoch, err = curView.getRewardInstByEpoch(); err != nil {
			return blockchain.NewBlockChainError(blockchain.BuildRewardInstructionError, err)
		}
	}

	//build beacon committee change instruction
	if state.isEndEpoch {
		var err error
		if state.beaconSwapInstruction, err = curView.buildChangeBeaconValidatorByEpoch(); err != nil {
			panic(err)
		}
	}

	//build random instruction
	if state.isGettingRandomNumber {
		if err := s.buildRandomInstruction(); err != nil {
			panic(err)
		}
	}

	//build assign instruction
	if curView.IsGettingRandomNumber && len(state.randomInstruction) >= 1 {
		if err := s.buildAsssignInstruction(); err != nil {
			panic(err)
		}
	}
	return nil
}

func (s *BeaconCoreApp) buildInstructionFromShardAction() error {
	state := s.CreateState
	curView := state.curView

	//build staking & auto staking instruction & shard committee instruction
	validStakeInstructions := [][]string{}
	validStakePublicKeys := []string{}
	validStopAutoStakingInstructions := [][]string{}
	validSwapInstructions := make(map[byte][][]string)
	acceptedRewardInstructions := [][]string{}

	for shardID, shardBlocks := range state.s2bBlks {
		Logger.log.Infof("Beacon Producer Got %+v Shard Block from shard %+v: ", len(shardBlocks), shardID)
		for _, shardBlock := range shardBlocks {
			validStakeInstruction, tempValidStakePublicKeys, validSwapInstruction, acceptedRewardInstruction, stopAutoStakingInstruction := buildInstructionFromBlock(shardBlock, curView)

			validStakeInstructions = append(validStakeInstructions, validStakeInstruction...)
			validSwapInstructions[shardID] = append(validSwapInstructions[shardID], validSwapInstruction[shardID]...)
			acceptedRewardInstructions = append(acceptedRewardInstructions, acceptedRewardInstruction)
			validStopAutoStakingInstructions = append(validStopAutoStakingInstructions, stopAutoStakingInstruction...)
			validStakePublicKeys = append(validStakePublicKeys, tempValidStakePublicKeys...)
		}
	}

	s.CreateState.validStakeInstructions = validStakeInstructions
	s.CreateState.validSwapInstructions = validSwapInstructions
	s.CreateState.acceptedRewardInstructions = acceptedRewardInstructions
	s.CreateState.validStopAutoStakingInstructions = validStopAutoStakingInstructions
	return nil
}

func (s *BeaconCoreApp) buildHeader() error {
	curView := s.CreateState.curView

	newBlock := s.CreateState.newBlock
	newBlock.Header = BeaconHeader{}

	//======Build Header Essential Data=======
	newBlock.Header.Version = blockchain.BEACON_BLOCK_VERSION
	newBlock.Header.Height = curView.GetHeight() + 1
	if s.CreateState.isNewEpoch {
		newBlock.Header.Epoch = curView.GetEpoch() + 1
	}
	newBlock.Header.ConsensusType = common.BlsConsensus2
	newBlock.Header.Producer = s.CreateState.proposer
	newBlock.Header.ProducerPubKeyStr = s.CreateState.proposer

	newBlock.Header.Timestamp = s.CreateState.createTimeStamp
	newBlock.Header.TimeSlot = s.CreateState.createTimeSlot
	newBlock.Header.PreviousBlockHash = *curView.GetBlock().Hash()

	//============Build Header Hash=============
	// create new view
	newViewInterface, err := curView.CreateNewViewFromBlock(newBlock)
	if err != nil {
		return err
	}
	newView := newViewInterface.(*BeaconView)
	// BeaconValidator root: beacon committee + beacon pending committee
	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconCommittee)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	validatorArr := append([]string{}, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconPendingValidator)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	validatorArr = append(validatorArr, beaconPendingValidatorStr...)
	tempBeaconCommitteeAndValidatorRoot, err := GenerateHashFromStringArray(validatorArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateBeaconCommitteeAndValidatorRootError, err)
	}
	// BeaconCandidate root: beacon current candidate + beacon next candidate
	beaconCandidateArr := append(newView.CandidateBeaconWaitingForCurrentRandom, newView.CandidateBeaconWaitingForNextRandom...)

	beaconCandidateArrStr, err := incognitokey.CommitteeKeyListToString(beaconCandidateArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	tempBeaconCandidateRoot, err := GenerateHashFromStringArray(beaconCandidateArrStr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateBeaconCandidateRootError, err)
	}
	// Shard candidate root: shard current candidate + shard next candidate
	shardCandidateArr := append(newView.CandidateShardWaitingForCurrentRandom, newView.CandidateShardWaitingForNextRandom...)

	shardCandidateArrStr, err := incognitokey.CommitteeKeyListToString(shardCandidateArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	tempShardCandidateRoot, err := GenerateHashFromStringArray(shardCandidateArrStr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateShardCandidateRootError, err)
	}
	// Shard Validator root
	shardPendingValidator := make(map[byte][]string)
	for shardID, keys := range newView.ShardPendingCommittee {
		keysStr, err := incognitokey.CommitteeKeyListToString(keys)
		if err != nil {
			return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
		}
		shardPendingValidator[shardID] = keysStr
	}

	shardCommittee := make(map[byte][]string)
	for shardID, keys := range newView.ShardCommittee {
		keysStr, err := incognitokey.CommitteeKeyListToString(keys)
		if err != nil {
			return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
		}
		shardCommittee[shardID] = keysStr
	}

	tempShardCommitteeAndValidatorRoot, err := GenerateHashFromMapByteString(shardPendingValidator, shardCommittee)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateShardCommitteeAndValidatorRootError, err)
	}

	tempAutoStakingRoot, err := GenerateHashFromMapStringBool(newView.AutoStaking)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.AutoStakingRootHashError, err)
	}
	// Shard state hash
	tempShardStateHash, err := GenerateHashFromShardState(s.CreateState.shardStates)
	if err != nil {
		Logger.log.Error(err)
		return blockchain.NewBlockChainError(blockchain.GenerateShardStateError, err)
	}
	// Instruction Hash
	tempInstructionArr := []string{}
	for _, strs := range s.CreateState.newBlock.Body.Instructions {
		tempInstructionArr = append(tempInstructionArr, strs...)
	}
	tempInstructionHash, err := GenerateHashFromStringArray(tempInstructionArr)
	if err != nil {
		Logger.log.Error(err)
		return blockchain.NewBlockChainError(blockchain.GenerateInstructionHashError, err)
	}
	// Instruction merkle root
	flattenInsts, err := blockchain.FlattenAndConvertStringInst(s.CreateState.newBlock.Body.Instructions)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.FlattenAndConvertStringInstError, err)
	}
	// add hash to header
	newBlock.Header.BeaconCommitteeAndValidatorRoot = tempBeaconCommitteeAndValidatorRoot
	newBlock.Header.BeaconCandidateRoot = tempBeaconCandidateRoot
	newBlock.Header.ShardCandidateRoot = tempShardCandidateRoot
	newBlock.Header.ShardCommitteeAndValidatorRoot = tempShardCommitteeAndValidatorRoot
	newBlock.Header.ShardStateHash = tempShardStateHash
	newBlock.Header.InstructionHash = tempInstructionHash
	newBlock.Header.AutoStakingRoot = tempAutoStakingRoot
	copy(newBlock.Header.InstructionMerkleRoot[:], blockchain.GetKeccak256MerkleRoot(flattenInsts))
	newBlock.Header.Timestamp = time.Now().Unix()
	return nil
}

func (BeaconCoreApp) createNewViewFromBlock(curView *BeaconView, block *BeaconBlock, newView *BeaconView) error {
	return nil
}

func (BeaconCoreApp) preValidate() error {
	return nil
}
