package shard

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
)

type ShardApp interface {
	//create block
	preCreateBlock(state *CreateNewBlockState) error
	buildTxFromCrossShard(state *CreateNewBlockState) error             // build tx from crossshard
	buildTxFromMemPool(state *CreateNewBlockState) error                // build tx from mempool
	buildResponseTxFromTxWithMetadata(state *CreateNewBlockState) error // build tx from metadata tx
	processBeaconInstruction(state *CreateNewBlockState) error          // execute beacon instruction & build tx if any
	generateInstruction(state *CreateNewBlockState) error               //create block instruction
	buildHeader(state *CreateNewBlockState) error

	//crete view from block
	createNewViewFromBlock(state *CreateNewBlockState) error

	//validate block
	preValidate(state *ValidateBlockState) error

	//store block
	storeDatabase(state *StoreDatabaseState) error
}

type BeaconBlockInterface interface {
	GetConfirmedCrossShardBlockToShard() map[byte]map[byte][]*CrossShardBlock
}

type AppData struct {
	Logger      common.Logger
	CreateBlock struct {
		crossShardTx             map[byte][]blockchain.CrossTransaction
		txToRemove               []metadata.Transaction
		txsToAdd                 []metadata.Transaction
		txsFromMetadataTx        []metadata.Transaction
		txsFromBeaconInstruction []metadata.Transaction
		errInstruction           [][]string
		stakingTx                map[string]string
		newShardPendingValidator []string
		instruction              [][]string
	}
}

type DB interface {
	GetGenesisBlock() consensus.BlockInterface
	//GetBeaconBlockHashByIndex(uint64) (common.Hash, error)
	//FetchBeaconBlock(common.Hash) ([]byte, error)
}

type BlockChain interface {
	//GetDB() DB
	GetCurrentBeaconHeight() (uint64, error) //get final confirm beacon block height
	GetCurrentEpoch() (uint64, error)        //get final confirm beacon block height
	GetChainParams() blockchain.Params

	ValidateCrossShardBlock(block *CrossShardBlock) error

	GetCrossShardPool(shardID byte) blockchain.CrossShardPool
	GetLatestCrossShard(from byte, to byte) uint64
	GetNextCrossShard(from byte, to byte, startHeight uint64) uint64

	GetAllValidCrossShardBlockFromPool(toShard byte) map[byte][]*CrossShardBlock
	GetValidBeaconBlockFromPool() []BeaconBlockInterface
	GetPendingTransaction(shardID byte) (txsToAdd []metadata.Transaction, txToRemove []metadata.Transaction, totalFee uint64)

	GetShardPendingCommittee(shardID byte) []incognitokey.CommitteePublicKey
	//GetShardCommittee(shardID byte) []incognitokey.CommitteePublicKey
	GetTransactionByHash(hash common.Hash) (byte, common.Hash, int, metadata.Transaction, error)
	FetchAutoStakingByHeight(uint64) (map[string]bool, error)
}

type FakeBC struct {
}

func (FakeBC) GetValidBeaconBlockFromPool() []BeaconBlockInterface {
	panic("implement me")
}

func (FakeBC) GetShardPendingCommittee(shardID byte) []incognitokey.CommitteePublicKey {
	return []incognitokey.CommitteePublicKey{}
}

func (FakeBC) GetCurrentBeaconHeight() (uint64, error) {
	return 1, nil
}

func (FakeBC) GetCurrentEpoch() (uint64, error) {
	panic("implement me")
}

func (FakeBC) GetChainParams() blockchain.Params {
	return blockchain.Params{
		Epoch: 10,
	}
}

func (FakeBC) GetCrossShardPool(shardID byte) blockchain.CrossShardPool {
	return mempool.GetCrossShardPool(shardID)
}

func (FakeBC) GetLatestCrossShard(from byte, to byte) uint64 {
	return 1
}

func (FakeBC) GetNextCrossShard(from byte, to byte, startHeight uint64) uint64 {
	return 1
}

func (FakeBC) GetAllValidCrossShardBlockFromPool(toShard byte) map[byte][]*CrossShardBlock {
	return nil
}

func (FakeBC) ValidateCrossShardBlock(block *CrossShardBlock) error {
	return nil
}

func (FakeBC) GetPendingTransaction(shardID byte) (txsToAdd []metadata.Transaction, txToRemove []metadata.Transaction, totalFee uint64) {
	return []metadata.Transaction{}, []metadata.Transaction{}, uint64(0)
}

func (FakeBC) GetTransactionByHash(hash common.Hash) (byte, common.Hash, int, metadata.Transaction, error) {
	return 0, common.Hash{}, 0, nil, nil
}

func (FakeBC) FetchAutoStakingByHeight(uint64) (map[string]bool, error) {
	return map[string]bool{}, nil
}
