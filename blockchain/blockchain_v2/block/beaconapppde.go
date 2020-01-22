package block

import "github.com/incognitochain/incognito-chain/common"

type BeaconPDEApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconPDEApp) preCreateBlock() error {
	state := s.CreateState
	curView := state.curView

	//TODO: get s2b blocks from pool => s2bBlks
	// -> Only accept block in one epoch
	state.s2bBlks = make(map[byte][]*ShardToBeaconBlock)

	//newEpoch? endEpoch? finalBlockInEpoch?
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == 1 {
		state.isNewEpoch = true
	}
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == 0 {
		state.isEndEpoch = true
	}
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == curView.BC.GetChainParams().RandomTime {
		state.isRandomTime = true
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

func (s *BeaconPDEApp) buildInstructionByEpoch() error {
	return nil
}

func (s *BeaconPDEApp) buildInstructionFromShardAction() error {
	return nil
}

func (s *BeaconPDEApp) buildHeader() error {
	return nil
}

func (s *BeaconPDEApp) updateNewViewFromBlock(block *BeaconBlock) error {
	return nil
}

func (s *BeaconPDEApp) preValidate() error {
	return nil
}
