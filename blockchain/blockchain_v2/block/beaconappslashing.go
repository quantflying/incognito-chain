package block

import "github.com/incognitochain/incognito-chain/common"

type BeaconSlashingApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
	StoreState    *StoreBeaconDatabaseState
}

func (s *BeaconSlashingApp) preCreateBlock() error {
	return nil
}
func (s *BeaconSlashingApp) buildTxFromCrossShard() error {
	return nil
}
func (s *BeaconSlashingApp) buildTxFromMemPool() error {
	return nil
}
func (s *BeaconSlashingApp) buildResponseTxFromTxWithMetadata() error {
	return nil
}
func (s *BeaconSlashingApp) processBeaconInstruction() error {
	return nil
}
func (s *BeaconSlashingApp) generateInstruction() error {
	return nil
}

func (s *BeaconSlashingApp) buildHeader() error {
	return nil
}

//crete view from block
func (s *BeaconSlashingApp) updateNewViewFromBlock(block *ShardBlock) error {
	return nil
}

//validate block
func (s *BeaconSlashingApp) preValidate() error {
	return nil
}

//store block
func (s *BeaconSlashingApp) storeDatabase(state *StoreShardDatabaseState) error {
	return nil
}
