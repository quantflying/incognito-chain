package block

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
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
	if curView.GetHeight()+1%curView.BC.GetChainParams().Epoch > curView.BC.GetChainParams().RandomTime && !state.isGettingRandomNumber {

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

func (BeaconCoreApp) buildHeader() error {
	return nil
}

func (BeaconCoreApp) createNewViewFromBlock(curView *BeaconView, block *BeaconBlock, newView *BeaconView) error {
	return nil
}

func (BeaconCoreApp) preValidate() error {
	return nil
}
