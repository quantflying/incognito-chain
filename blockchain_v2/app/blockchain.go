package app

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/btc"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
)

// blockchainV2 maintain local information of all beacon, shard
// config and other global object
//TODO: add finalized chain information to blockchainV2
type blockchainV2 struct {
	chainParams  *blockchain.Params
	randomClient btc.RandomClient
	activeShard  int
}

func (bc *blockchainV2) GetActiveShard() int {
	return bc.activeShard
}

func (bc *blockchainV2) GetShardIDs() []int {
	shardIDs := []int{}
	for i := 0; i < bc.GetActiveShard(); i++ {
		shardIDs = append(shardIDs, i)
	}
	return shardIDs
}

func (bc *blockchainV2) GetCurrentBeaconHeight() uint64 {
	//TODO: implement later
	return 1
}

func (bc *blockchainV2) GetAllValidCrossShardBlockFromPool(toShard byte) map[byte][]blockinterface.CrossShardBlockInterface {
	//TODO: implement later
	return nil
}

func (bc *blockchainV2) GetCrossShardPool(shardID byte) blockchain.CrossShardPool {
	//TODO: implement later
	return mempool.GetCrossShardPool(shardID)
}

func (bc *blockchainV2) GetLatestCrossShard(from byte, to byte) uint64 {
	//TODO: implement later
	return 1
}

func (bc *blockchainV2) GetNextCrossShard(from byte, to byte, startHeight uint64) uint64 {
	//TODO: implement later
	return 1
}

func (bc *blockchainV2) ValidateCrossShardBlock(block blockinterface.CrossShardBlockInterface) error {
	//TODO: implement later
	return nil
}

func (bc *blockchainV2) GetPendingTransaction(shardID byte) (txsToAdd []metadata.Transaction, txToRemove []metadata.Transaction, totalFee uint64) {
	//TODO: implement later
	return []metadata.Transaction{}, []metadata.Transaction{}, uint64(0)
}

func (bc *blockchainV2) GetValidBeaconBlockFromPool() []blockinterface.BeaconBlockInterface {
	panic("implement me")
}

func (bc *blockchainV2) GetTransactionByHash(hash common.Hash) (byte, common.Hash, int, metadata.Transaction, error) {
	//TODO: implement later
	return 0, common.Hash{}, 0, nil, nil
}

func (bc *blockchainV2) FetchAutoStakingByHeight(uint64) (map[string]bool, error) {
	//TODO: implement later
	return map[string]bool{}, nil
}

func (bc *blockchainV2) GetStakingAmountShard() uint64 {
	panic("implement me")
}

func (bc *blockchainV2) GetTxChainHeight(tx metadata.Transaction) (uint64, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetChainHeight(byte) uint64 {
	panic("implement me")
}

func (bc *blockchainV2) GetBeaconHeight() uint64 {
	panic("implement me")
}

func (bc *blockchainV2) GetCurrentBeaconBlockHeight(byte) uint64 {
	panic("implement me")
}

func (bc *blockchainV2) GetAllCommitteeValidatorCandidate() (map[byte][]incognitokey.CommitteePublicKey, map[byte][]incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetAllCommitteeValidatorCandidateFlattenListFromDatabase() ([]string, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetStakingTx(byte) map[string]string {
	panic("implement me")
}

func (bc *blockchainV2) GetAutoStakingList() map[string]bool {
	panic("implement me")
}

func (bc *blockchainV2) GetTxValue(txid string) (uint64, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetShardIDFromTx(txid string) (byte, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetCentralizedWebsitePaymentAddress() string {
	panic("implement me")
}

func (bc *blockchainV2) ListPrivacyTokenAndBridgeTokenAndPRVByShardID(byte) ([]common.Hash, error) {
	panic("implement me")
}

func (bc *blockchainV2) GetBeaconHeightBreakPointBurnAddr() uint64 {
	panic("implement me")
}

func (bc *blockchainV2) GetBurningAddress(blockHeight uint64) string {
	panic("implement me")
}

func (bc *blockchainV2) GetShardRewardStateDB(shardID byte) *statedb.StateDB {
	panic("implement me")
}

func (bc *blockchainV2) GetShardFeatureStateDB(shardID byte) *statedb.StateDB {
	panic("implement me")
}

func (bc *blockchainV2) GetBeaconFeatureStateDB() *statedb.StateDB {
	panic("implement me")
}

func (bc *blockchainV2) GetBeaconRewardStateDB() *statedb.StateDB {
	panic("implement me")
}

func (bc *blockchainV2) GetBeaconSlashStateDB() *statedb.StateDB {
	panic("implement me")
}
