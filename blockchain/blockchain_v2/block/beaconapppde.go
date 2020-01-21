package block

import "github.com/incognitochain/incognito-chain/common"

type BeaconPDEApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconPDEApp) preCreateBlock() error {
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
