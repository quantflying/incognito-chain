package blockchain_v2

import (
	"time"

	"github.com/incognitochain/incognito-chain/blockchain_v2/btc"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/pubsub"
)

// config is a descriptor which specifies the blockchain instance configuration.
type Config struct {
	DataBase          incdb.Database
	MemCache          *memcache.MemoryCache
	Interrupt         <-chan struct{}
	ChainParams       *Params
	RelayShards       []byte
	NodeMode          string
	ShardToBeaconPool ShardToBeaconPoolInterface
	CrossShardPool    map[byte]CrossShardPoolInterface
	BeaconPool        BeaconPoolInterface
	ShardPool         map[byte]ShardPoolInterface
	TxPool            TxPoolInterface
	TempTxPool        TxPoolInterface
	CRemovedTxs       chan metadata.Transaction
	FeeEstimator      map[byte]FeeEstimator
	IsBlockGenStarted bool
	PubSubManager     *pubsub.PubSubManager
	RandomClient      btc.RandomClient
	Server            interface {
		BoardcastNodeState() error
		PublishNodeState(userLayer string, shardID int) error

		PushMessageGetBlockBeaconByHeight(from uint64, to uint64) error
		PushMessageGetBlockBeaconByHash(blksHash []common.Hash, getFromPool bool, peerID libp2p.ID) error
		PushMessageGetBlockBeaconBySpecificHeight(heights []uint64, getFromPool bool) error

		PushMessageGetBlockShardByHeight(shardID byte, from uint64, to uint64) error
		PushMessageGetBlockShardByHash(shardID byte, blksHash []common.Hash, getFromPool bool, peerID libp2p.ID) error
		PushMessageGetBlockShardBySpecificHeight(shardID byte, heights []uint64, getFromPool bool) error

		PushMessageGetBlockShardToBeaconByHeight(shardID byte, from uint64, to uint64) error
		PushMessageGetBlockShardToBeaconByHash(shardID byte, blksHash []common.Hash, getFromPool bool, peerID libp2p.ID) error
		PushMessageGetBlockShardToBeaconBySpecificHeight(shardID byte, blksHeight []uint64, getFromPool bool, peerID libp2p.ID) error

		PushMessageGetBlockCrossShardByHash(fromShard byte, toShard byte, blksHash []common.Hash, getFromPool bool, peerID libp2p.ID) error
		PushMessageGetBlockCrossShardBySpecificHeight(fromShard byte, toShard byte, blksHeight []uint64, getFromPool bool, peerID libp2p.ID) error
		UpdateConsensusState(role string, userPbk string, currentShard *byte, beaconCommittee []string, shardCommittee map[byte][]string)
		PushBlockToAll(block blockinterface.BlockInterface, isBeacon bool) error
	}
	// UserKeySet *incognitokey.KeySet

	ConsensusEngine interface {
		ValidateProducerSig(block blockinterface.BlockInterface, consensusType string) error
		ValidateBlockCommitteSig(block blockinterface.BlockInterface, committee []incognitokey.CommitteePublicKey, consensusType string) error
		GetCurrentMiningPublicKey() (string, string)
		GetMiningPublicKeyByConsensus(consensusName string) (string, error)
		GetUserLayer() (string, int)
		GetUserRole() (string, string, int)
		IsOngoing(chainName string) bool
		CommitteeChange(chainName string)
	}

	Highway interface {
		BroadcastCommittee(uint64, []incognitokey.CommitteePublicKey, map[byte][]incognitokey.CommitteePublicKey, map[byte][]incognitokey.CommitteePublicKey)
	}
}

type ShardToBeaconPoolInterface interface {
	RemoveBlock(map[byte]uint64)
	//GetFinalBlock() map[byte][]ShardToBeaconBlock
	AddShardToBeaconBlock(blockinterface.ShardToBeaconBlockInterface) (uint64, uint64, error)
	//ValidateShardToBeaconBlock(ShardToBeaconBlock) error
	GetValidBlockHash() map[byte][]common.Hash
	GetValidBlock(map[byte]uint64) map[byte][]blockinterface.ShardToBeaconBlockInterface
	GetValidBlockHeight() map[byte][]uint64
	GetLatestValidPendingBlockHeight() map[byte]uint64
	GetBlockByHeight(shardID byte, height uint64) blockinterface.ShardToBeaconBlockInterface
	SetShardState(map[byte]uint64)
	GetAllBlockHeight() map[byte][]uint64
	RevertShardToBeaconPool(s byte, height uint64)
}

type CrossShardPoolInterface interface {
	AddCrossShardBlock(blockinterface.CrossShardBlockInterface) (map[byte]uint64, byte, error)
	GetValidBlock(map[byte]uint64) map[byte][]blockinterface.CrossShardBlockInterface
	GetLatestValidBlockHeight() map[byte]uint64
	GetValidBlockHeight() map[byte][]uint64
	GetBlockByHeight(_shardID byte, height uint64) blockinterface.CrossShardBlockInterface
	RemoveBlockByHeight(map[byte]uint64)
	UpdatePool() map[byte]uint64
	GetAllBlockHeight() map[byte][]uint64
	RevertCrossShardPool(uint64)
}

type ShardPoolInterface interface {
	RemoveBlock(height uint64)
	AddShardBlock(block blockinterface.ShardBlockInterface) error
	GetValidBlockHash() []common.Hash
	GetValidBlock() []blockinterface.ShardBlockInterface
	GetValidBlockHeight() []uint64
	GetLatestValidBlockHeight() uint64
	SetShardState(height uint64)
	RevertShardPool(uint64)
	GetAllBlockHeight() []uint64
	GetPendingBlockHeight() []uint64
	Start(chan struct{})
}

type BeaconPoolInterface interface {
	RemoveBlock(height uint64)
	AddBeaconBlock(block blockinterface.BeaconBlockInterface) error
	GetValidBlock() []blockinterface.BeaconBlockInterface
	GetValidBlockHeight() []uint64
	SetBeaconState(height uint64)
	GetBeaconState() uint64
	RevertBeconPool(height uint64)
	GetAllBlockHeight() []uint64
	Start(chan struct{})
	GetPendingBlockHeight() []uint64
}
type TxPoolInterface interface {
	// LastUpdated returns the last time a transaction was added to or
	// removed from the source pool.
	LastUpdated() time.Time
	// MiningDescs returns a slice of mining descriptors for all the
	// transactions in the source pool.
	MiningDescs() []*metadata.TxDesc
	// HaveTransaction returns whether or not the passed transaction hash
	// exists in the source pool.
	HaveTransaction(hash *common.Hash) bool
	// RemoveTx remove tx from tx resource
	RemoveTx(txs []metadata.Transaction, isInBlock bool)
	RemoveCandidateList([]string)
	EmptyPool() bool
	MaybeAcceptTransactionForBlockProducing(metadata.Transaction, int64) (*metadata.TxDesc, error)
	ValidateTxList(txs []metadata.Transaction) error
	//CheckTransactionFee
	// CheckTransactionFee(tx metadata.Transaction) (uint64, error)
	// Check tx validate by it self
	// ValidateTxByItSelf(tx metadata.Transaction) bool
}

type FeeEstimatorInterface interface {
	RegisterBlock(block blockinterface.ShardBlockInterface) error
}
