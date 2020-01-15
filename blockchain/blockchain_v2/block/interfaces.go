package block

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/btc"
	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/database"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"time"
)

type ShardApp interface {
	//create block
	preCreateBlock() error
	buildTxFromCrossShard() error             // build tx from crossshard
	buildTxFromMemPool() error                // build tx from mempool
	buildResponseTxFromTxWithMetadata() error // build tx from metadata tx
	processBeaconInstruction() error          // execute beacon instruction & build tx if any
	generateInstruction() error               //create block instruction
	buildHeader() error

	//crete view from block
	updateNewViewFromBlock(block *ShardBlock) error

	//validate block
	preValidate() error

	//store block
	storeDatabase(state *StoreDatabaseState) error
}

type BeaconApp interface {
	//create block
	preCreateBlock() error
	buildInstructionByEpoch() error
	buildInstructionFromShardAction() error

	buildHeader() error

	//crete view from block
	updateNewViewFromBlock(block *BeaconBlock) error

	//validate block
	preValidate() error

	////store block
	//storeDatabase(state *StoreDatabaseState) error
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
	GetAllTokenIDForReward(epoch uint64) ([]common.Hash, error)
	GetRewardOfShardByEpoch(epoch uint64, shardID byte, tokenID common.Hash) (uint64, error)
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
	FetchAutoStakingByHeight(uint64) (map[string]bool, error)

	GetCommitteeReward(committeeAddress []byte, tokenID common.Hash) (uint64, error)
	InitTxSalaryByCoinID(payToAddress *privacy.PaymentAddress, amount uint64, payByPrivateKey *privacy.PrivateKey, meta metadata.Metadata, coinID common.Hash, shardID byte) (metadata.Transaction, error)
	metadata.BlockchainRetriever

	GetRandomClient() btc.RandomClient
}

type FakeRandomClient struct{}

func (FakeRandomClient) GetNonceByTimestamp(startTime time.Time, maxTime time.Duration, timestamp int64) (int, int64, int64, error) {
	panic("implement me")
}

func (FakeRandomClient) VerifyNonceWithTimestamp(startTime time.Time, maxTime time.Duration, timestamp int64, nonce int64) (bool, error) {
	panic("implement me")
}

func (FakeRandomClient) GetCurrentChainTimeStamp() (int64, error) {
	return time.Now().Unix(), nil
}

func (FakeRandomClient) GetTimeStampAndNonceByBlockHeight(blockHeight int) (int64, int64, error) {
	panic("implement me")
}

type FakeBC struct {
}

func (FakeBC) GetRandomClient() btc.RandomClient {
	return FakeRandomClient{}
}

func (FakeBC) GetBeaconHeightBreakPointBurnAddr() uint64 {
	panic("implement me")
}

func (FakeBC) GetBurningAddress(blockHeight uint64) string {
	panic("implement me")
}

func (FakeBC) InitTxSalaryByCoinID(payToAddress *privacy.PaymentAddress, amount uint64, payByPrivateKey *privacy.PrivateKey, meta metadata.Metadata, coinID common.Hash, shardID byte) (metadata.Transaction, error) {
	return nil, nil
}
func (FakeBC) GetCommitteeReward(committeeAddress []byte, tokenID common.Hash) (uint64, error) {
	return 0, nil
}

func (FakeBC) GetStakingAmountShard() uint64 {
	panic("implement me")
}

func (FakeBC) GetTxChainHeight(tx metadata.Transaction) (uint64, error) {
	panic("implement me")
}

func (FakeBC) GetChainHeight(byte) uint64 {
	panic("implement me")
}

func (FakeBC) GetBeaconHeight() uint64 {
	panic("implement me")
}

func (FakeBC) GetCurrentBeaconBlockHeight(byte) uint64 {
	panic("implement me")
}

func (FakeBC) GetAllCommitteeValidatorCandidate() (map[byte][]incognitokey.CommitteePublicKey, map[byte][]incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, error) {
	panic("implement me")
}

func (FakeBC) GetAllCommitteeValidatorCandidateFlattenListFromDatabase() ([]string, error) {
	panic("implement me")
}

func (FakeBC) GetStakingTx(byte) map[string]string {
	panic("implement me")
}

func (FakeBC) GetAutoStakingList() map[string]bool {
	panic("implement me")
}

func (FakeBC) GetDatabase() database.DatabaseInterface {
	panic("implement me")
}

func (FakeBC) GetTxValue(txid string) (uint64, error) {
	panic("implement me")
}

func (FakeBC) GetShardIDFromTx(txid string) (byte, error) {
	panic("implement me")
}

func (FakeBC) GetCentralizedWebsitePaymentAddress() string {
	panic("implement me")
}

func (FakeBC) GetAllCoinID() ([]common.Hash, error) {
	panic("implement me")
}

func (FakeBC) GetValidBeaconBlockFromPool() []BeaconBlockInterface {
	panic("implement me")
}

func (FakeBC) GetShardPendingCommittee(shardID byte) []incognitokey.CommitteePublicKey {
	return []incognitokey.CommitteePublicKey{}
}

//look at beacon chain, get the final view and get height
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
