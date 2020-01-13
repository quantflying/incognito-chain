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

	//newEpoch? endEpoch?
	if curView.GetHeight()+1%curView.BC.GetChainParams().Epoch == 1 {
		state.isNewEpoch = true
	}
	if curView.GetHeight()+1%curView.BC.GetChainParams().Epoch == 0 {
		state.isEndEpoch = true
	}
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
	}

	//build staking & auto staking instruction

	//build random instruction

	return nil
}

func (s *BeaconCoreApp) buildInstructionFromShardAction() error {
	state := s.CreateState
	curView := state.curView

	//build shard committee change instruction
	for shardID, shardBlocks := range state.s2bBlks {
		Logger.log.Infof("Beacon Producer Got %+v Shard Block from shard %+v: ", len(shardBlocks), shardID)
		for _, shardBlock := range shardBlocks {
			if err := s.buildInstructionFromBlock(shardBlock); err != nil {
				return err
			}
		}
	}
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
