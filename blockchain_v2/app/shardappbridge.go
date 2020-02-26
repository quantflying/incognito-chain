package app

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
)

type ShardBridgeApp struct {
	Logger        common.Logger
	createState   *CreateShardBlockState
	validateState *ValidateShardBlockState
	storeState    *StoreShardDatabaseState
	storeSuccess  bool
}

func (sba *ShardBridgeApp) preCreateBlock() error {
	return nil
}
func (sba *ShardBridgeApp) buildTxFromCrossShard() error {
	return nil
}
func (sba *ShardBridgeApp) buildTxFromMemPool() error {
	return nil
}
func (sba *ShardBridgeApp) buildResponseTxFromTxWithMetadata() error {
	return nil
}
func (sba *ShardBridgeApp) processBeaconInstruction() error {
	return nil
}
func (sba *ShardBridgeApp) generateInstruction() error {
	return nil
}

func (sba *ShardBridgeApp) buildHeader() error {
	return nil
}

//crete view from block
func (sba *ShardBridgeApp) updateNewViewFromBlock(block blockinterface.ShardBlockInterface) error {
	return nil
}

//validate block
func (sba *ShardBridgeApp) preValidate() error {
	return nil
}

//store block
func (sba *ShardBridgeApp) storeDatabase(state *StoreShardDatabaseState) error {
	var err error
	err = storeBurningConfirm(sba.storeState.curView.featureStateDB, state.block)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.StoreBurningConfirmError, err)
	}
	err = updateBridgeIssuanceStatus(sba.storeState.curView.featureStateDB, state.block)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UpdateBridgeIssuanceStatusError, err)
	}
	return nil
}
