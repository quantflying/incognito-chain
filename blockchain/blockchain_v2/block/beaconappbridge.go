package block

import "github.com/incognitochain/incognito-chain/common"

type BeaconBridgeApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconBridgeApp) preCreateBlock() error {
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

func (s *BeaconBridgeApp) buildInstructionByEpoch() error {
	return nil
}

func (s *BeaconBridgeApp) buildInstructionFromShardAction() error {
	db := s.ValidateState.bc.GetDatabase()
	newBeaconHeight := s.ValidateState.curView.GetHeight() + 1
	statefulActionsByShardID := map[byte][][]string{}

	for shardID, shardBlocks := range s.CreateState.s2bBlks {
		for _, block := range shardBlocks {
			bridgeInstructionForBlock, err := buildBridgeInstructions(
				shardID,
				block.Instructions,
				newBeaconHeight,
				db,
				s.Logger,
			)
			if err != nil {
				s.Logger.Errorf("Build bridge instructions failed: %s", err.Error())
			}
			// Pick instruction with shard committee's pubkeys to save to beacon block
			confirmInsts := pickBridgeSwapConfirmInst(block)
			if len(confirmInsts) > 0 {
				bridgeInstructionForBlock = append(bridgeInstructionForBlock, confirmInsts...)
				s.Logger.Infof("Beacon block %d found bridge swap confirm inst in shard block %d: %s", newBeaconHeight, block.Header.Height, confirmInsts)
			}
			// Collect stateful actions
			statefulActions := collectStatefulActions(block.Instructions)
			// group stateful actions by shardID
			_, found := statefulActionsByShardID[shardID]
			if !found {
				statefulActionsByShardID[shardID] = statefulActions
			} else {
				statefulActionsByShardID[shardID] = append(statefulActionsByShardID[shardID], statefulActions...)
			}

			s.CreateState.bridgeInstructions = append(s.CreateState.bridgeInstructions, bridgeInstructionForBlock...)
		}
	}

	// build stateful instructions
	statefulInsts := buildStatefulInstructions(
		statefulActionsByShardID,
		newBeaconHeight,
		db,
		s.ValidateState.bc,
	)

	s.CreateState.statefulInstructions = append(s.CreateState.statefulInstructions, statefulInsts...)
	return nil
}

func (s *BeaconBridgeApp) buildHeader() error {
	return nil
}

func (s *BeaconBridgeApp) updateNewViewFromBlock(block *BeaconBlock) error {
	return nil
}

func (s *BeaconBridgeApp) preValidate() error {
	return nil
}
