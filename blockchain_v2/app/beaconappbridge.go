package app

import (
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
)

type BeaconBridgeApp struct {
	Logger        common.Logger
	createState   *CreateBeaconBlockState
	validateState *ValidateBeaconBlockState
	storeState    *StoreBeaconDatabaseState
	storeSuccess  bool
}

func (bba *BeaconBridgeApp) preCreateBlock() error {
	return nil
}

func (bba *BeaconBridgeApp) buildInstructionByEpoch() error {
	return nil
}

func (bba *BeaconBridgeApp) buildInstructionFromShardAction() error {
	if len(bba.createState.s2bBlks) == 0 {
		return nil
	}
	newBeaconHeight := bba.createState.curView.GetHeight() + 1

	for shardID, shardBlocks := range bba.createState.s2bBlks {
		for _, block := range shardBlocks {
			bridgeInstructionForBlock, err := buildBridgeInstructions(
				bba.createState.curView.GetCopiedFeatureStateDB(),
				shardID,
				block.GetInstructions(),
				newBeaconHeight,
			)
			if err != nil {
				bba.Logger.Errorf("Build bridge instructions failed: %bba", err.Error())
			}
			// Pick instruction with shard committee'bba pubkeys to save to beacon block
			confirmInsts := pickBridgeSwapConfirmInst(block)
			if len(confirmInsts) > 0 {
				bridgeInstructionForBlock = append(bridgeInstructionForBlock, confirmInsts...)
				bba.Logger.Infof("Beacon block %d found bridge swap confirm inst in shard block %d: %bba", newBeaconHeight, block.GetShardHeader().GetHeight(), confirmInsts)
			}

			bba.createState.bridgeInstructions = append(bba.createState.bridgeInstructions, bridgeInstructionForBlock...)
		}
	}
	return nil
}

func (bba *BeaconBridgeApp) buildHeader() error {
	return nil
}

func (bba *BeaconBridgeApp) updateNewViewFromBlock(block blockinterface.BeaconBlockInterface) error {
	return nil
}

func (bba *BeaconBridgeApp) preValidate() error {
	return nil
}

//==============================Save Database Logic===========================
func (bba *BeaconBridgeApp) storeDatabase() error {
	err := storeBridgeInstructions(bba.storeState.curView.featureStateDB, bba.storeState.block)
	if err != nil {
		return NewBlockChainError(ProcessBridgeInstructionError, err)
	}
	return nil
}
