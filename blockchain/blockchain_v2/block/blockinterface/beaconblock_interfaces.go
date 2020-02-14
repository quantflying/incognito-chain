package blockinterface

import (
	"github.com/incognitochain/incognito-chain/blockchain/blockchain_v2/block/shardstate"
	"github.com/incognitochain/incognito-chain/common"
)

type BeaconBlockInterface interface {
	GetHeader() BeaconHeaderInterface
	GetBody() BeaconBodyInterface
	GetVersion() int
	GetHash() *common.Hash
	// GetConfirmedCrossShardBlockToShard() map[byte]map[byte][]*CrossShardBlock

	// SetHeader(BeaconHeaderInterface) error
	// SetBody(BeaconBodyInterface) error
}

type BeaconHeaderInterface interface {
	GetTimestamp() int64
	GetMetaHash() common.Hash
	GetHash() *common.Hash
	GetVersion() int
	GetHeight() uint64
	GetEpoch() uint64
	GetConsensusType() string
	GetProducer() string
	GetPreviousBlockHash() common.Hash

	// SetTimestamp(int64) error
	// SetMetaHash(common.Hash) error
	// SetHash(common.Hash) error
	// SetVersion(int) error
	// SetHeight(uint64) error
	// SetEpoch(uint64) error
	// SetConsensusType(string) error
	// SetProducer(string) error
	// SetPreviousBlockHash(common.Hash) error
}

type BeaconHeaderV1Interface interface {
	BeaconHeaderInterface

	GetRound() int
	GetInstructionHash() common.Hash
	GetShardStateHash() common.Hash
	GetInstructionMerkleRoot() common.Hash
	GetBeaconCommitteeAndValidatorRoot() common.Hash
	GetBeaconCandidateRoot() common.Hash
	GetShardCandidateRoot() common.Hash
	GetShardCommitteeAndValidatorRoot() common.Hash
	GetAutoStakingRoot() common.Hash

	// SetRound(int) error
	// SetInstructionHash(common.Hash) error
	// SetShardStateHash(common.Hash) error
	// SetInstructionMerkleRoot(common.Hash) error
	// SetBeaconCommitteeAndValidatorRoot(common.Hash) error
	// SetBeaconCandidateRoot(common.Hash) error
	// SetShardCandidateRoot(common.Hash) error
	// SetShardCommitteeAndValidatorRoot(common.Hash) error
	// SetAutoStakingRoot(common.Hash) error
}

type BeaconBodyInterface interface {
	GetShardState() map[byte][]shardstate.ShardState
	GetInstructions() [][]string
	// SetShardState(map[byte][]beaconblockv1.ShardState) error
	// SetInstructions([][]string) error
}

type BeaconHeaderV2Interface interface {
	BeaconHeaderV1Interface

	GetTimeslot() uint64
	// SetRound(int) error
	// SetInstructionHash(common.Hash) error
	// SetShardStateHash(common.Hash) error
	// SetInstructionMerkleRoot(common.Hash) error
	// SetBeaconCommitteeAndValidatorRoot(common.Hash) error
	// SetBeaconCandidateRoot(common.Hash) error
	// SetShardCandidateRoot(common.Hash) error
	// SetShardCommitteeAndValidatorRoot(common.Hash) error
	// SetAutoStakingRoot(common.Hash) error
}
