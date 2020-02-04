package block

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database"
)

type BeaconBridgeApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconBridgeApp) preCreateBlock() error {
	return nil
}

func (s *BeaconBridgeApp) buildInstructionByEpoch() error {
	return nil
}

func (s *BeaconBridgeApp) buildInstructionFromShardAction() error {
	db := s.CreateState.bc.GetDatabase()
	newBeaconHeight := s.CreateState.curView.GetHeight() + 1

	if s.CreateState.s2bBlks == nil {
		return nil
	}

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

			s.CreateState.bridgeInstructions = append(s.CreateState.bridgeInstructions, bridgeInstructionForBlock...)
		}
	}
	return nil
}

func (s *BeaconBridgeApp) buildHeader() error {
	return nil
}

func (s *BeaconBridgeApp) updateNewViewFromBlock(block *BeaconBlock) error {
	//TODO: store db?
	batchPutData := []database.BatchData{}
	err := processBridgeInstructions(block, &batchPutData, s.ValidateState.bc, s.Logger)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.ProcessBridgeInstructionError, err)
	}
	return nil
}

func (s *BeaconBridgeApp) preValidate() error {
	return nil
}

//==============================Save Database Logic===========================
func (s *BeaconBridgeApp) storeDatabase(state *StoreBeaconDatabaseState) error {

	return nil
}
