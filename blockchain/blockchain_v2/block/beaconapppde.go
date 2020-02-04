package block

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database"
)

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
	db := s.CreateState.bc.GetDatabase()
	newBeaconHeight := s.CreateState.curView.GetHeight() + 1
	statefulActionsByShardID := map[byte][][]string{}

	if s.CreateState.s2bBlks == nil {
		return nil
	}

	for shardID, shardBlocks := range s.CreateState.s2bBlks {
		for _, block := range shardBlocks {
			// Collect stateful actions
			statefulActions := collectStatefulActions(block.Instructions)
			// group stateful actions by shardID
			_, found := statefulActionsByShardID[shardID]
			if !found {
				statefulActionsByShardID[shardID] = statefulActions
			} else {
				statefulActionsByShardID[shardID] = append(statefulActionsByShardID[shardID], statefulActions...)
			}
		}
	}

	// build stateful instructions
	statefulInsts := buildStatefulInstructions(
		statefulActionsByShardID,
		newBeaconHeight,
		db,
		s.CreateState.bc,
	)

	s.CreateState.statefulInstructions = append(s.CreateState.statefulInstructions, statefulInsts...)
	return nil
}

func (s *BeaconPDEApp) buildHeader() error {
	return nil
}

func (s *BeaconPDEApp) updateNewViewFromBlock(block *BeaconBlock) error {
	//TODO: store db?
	batchPutData := []database.BatchData{}
	// execute, store
	err := processPDEInstructions(block, &batchPutData, s.ValidateState.bc)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.ProcessPDEInstructionError, err)
	}
	return nil
}

func (s *BeaconPDEApp) preValidate() error {
	return nil
}

//==============================Save Database Logic===========================
func (s *BeaconPDEApp) storeDatabase(state *StoreBeaconDatabaseState) error {

	return nil
}
