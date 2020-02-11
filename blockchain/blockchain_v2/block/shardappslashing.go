package block

import (
	"github.com/incognitochain/incognito-chain/common"
)

type ShardSlashingApp struct {
	Logger        common.Logger
	CreateState   *CreateShardBlockState
	ValidateState *ValidateShardBlockState
	StoreState    *StoreShardDatabaseState
}

func (s *ShardSlashingApp) preCreateBlock() error {
	return nil
}
func (s *ShardSlashingApp) buildTxFromCrossShard() error {
	return nil
}
func (s *ShardSlashingApp) buildTxFromMemPool() error {
	return nil
}
func (s *ShardSlashingApp) buildResponseTxFromTxWithMetadata() error {
	return nil
}
func (s *ShardSlashingApp) processBeaconInstruction() error {
	return nil
}
func (s *ShardSlashingApp) generateInstruction() error {

	// maxShardCommitteeSize := blockchain.BestState.Shard[shardID].MaxShardCommitteeSize
	// minShardCommitteeSize := blockchain.BestState.Shard[shardID].MinShardCommitteeSize
	// maxShardCommitteeSize := 0
	// minShardCommitteeSize := 0
	// shardID := s.CreateState.curView.ShardID
	// shardCommittee, _ := incognitokey.CommitteeKeyListToString(s.CreateState.curView.ShardCommittee)

	// badProducersWithPunishment := buildBadProducersWithPunishment(false, int(shardID), shardCommittee, s.CreateState.bc)

	// swapInstruction, shardPendingValidator, shardCommittee, err := CreateSwapAction(shardPendingValidator, shardCommittee, maxShardCommitteeSize, minShardCommitteeSize, shardID, producersBlackList, badProducersWithPunishment, blockchain.config.ChainParams.Offset, blockchain.config.ChainParams.SwapOffset)
	// if err != nil {
	// 	Logger.log.Error(err)
	// 	return instructions, shardPendingValidator, shardCommittee, err
	// }
	return nil
}

func (s *ShardSlashingApp) buildHeader() error {
	return nil
}

//crete view from block
func (s *ShardSlashingApp) updateNewViewFromBlock(block *ShardBlock) error {
	return nil
}

//validate block
func (s *ShardSlashingApp) preValidate() error {
	return nil
}

//store block
func (s *ShardSlashingApp) storeDatabase(state *StoreShardDatabaseState) error {
	return nil
}
