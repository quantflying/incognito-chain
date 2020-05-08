package blockchain

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/incdb"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

// BestState houses information about the current best block and other info
// related to the state of the main chain as it exists from the point of view of
// the current best block.
//
// The BestSnapshot method can be used to obtain access to this information
// in a concurrent safe manner and the data will not be changed out from under
// the caller when chain state changes occur as the function name implies.
// However, the returned snapshot must be treated as immutable since it is
// shared by all callers.

type BeaconBestState struct {
	BestBlockHash                          common.Hash                                `json:"BestBlockHash"`         // The hash of the block.
	PreviousBestBlockHash                  common.Hash                                `json:"PreviousBestBlockHash"` // The hash of the block.
	BestBlock                              BeaconBlock                                `json:"BestBlock"`             // The block.
	BestShardHash                          map[byte]common.Hash                       `json:"BestShardHash"`
	BestShardHeight                        map[byte]uint64                            `json:"BestShardHeight"`
	Epoch                                  uint64                                     `json:"Epoch"`
	BeaconHeight                           uint64                                     `json:"BeaconHeight"`
	BeaconProposerIndex                    int                                        `json:"BeaconProposerIndex"`
	BeaconCommittee                        []incognitokey.CommitteePublicKey          `json:"BeaconCommittee"`
	BeaconPendingValidator                 []incognitokey.CommitteePublicKey          `json:"BeaconPendingValidator"`
	CandidateShardWaitingForCurrentRandom  []incognitokey.CommitteePublicKey          `json:"CandidateShardWaitingForCurrentRandom"` // snapshot shard candidate list, waiting to be shuffled in this current epoch
	CandidateBeaconWaitingForCurrentRandom []incognitokey.CommitteePublicKey          `json:"CandidateBeaconWaitingForCurrentRandom"`
	CandidateShardWaitingForNextRandom     []incognitokey.CommitteePublicKey          `json:"CandidateShardWaitingForNextRandom"` // shard candidate list, waiting to be shuffled in next epoch
	CandidateBeaconWaitingForNextRandom    []incognitokey.CommitteePublicKey          `json:"CandidateBeaconWaitingForNextRandom"`
	ShardCommittee                         map[byte][]incognitokey.CommitteePublicKey `json:"ShardCommittee"`        // current committee and validator of all shard
	ShardPendingValidator                  map[byte][]incognitokey.CommitteePublicKey `json:"ShardPendingValidator"` // pending candidate waiting for swap to get in committee of all shard
	AutoStaking                            map[string]bool                            `json:"AutoStaking"`
	CurrentRandomNumber                    int64                                      `json:"CurrentRandomNumber"`
	CurrentRandomTimeStamp                 int64                                      `json:"CurrentRandomTimeStamp"` // random timestamp for this epoch
	IsGetRandomNumber                      bool                                       `json:"IsGetRandomNumber"`
	Params                                 map[string]string                          `json:"Params,omitempty"`
	MaxBeaconCommitteeSize                 int                                        `json:"MaxBeaconCommitteeSize"`
	MinBeaconCommitteeSize                 int                                        `json:"MinBeaconCommitteeSize"`
	MaxShardCommitteeSize                  int                                        `json:"MaxShardCommitteeSize"`
	MinShardCommitteeSize                  int                                        `json:"MinShardCommitteeSize"`
	ActiveShards                           int                                        `json:"ActiveShards"`
	ConsensusAlgorithm                     string                                     `json:"ConsensusAlgorithm"`
	ShardConsensusAlgorithm                map[byte]string                            `json:"ShardConsensusAlgorithm"`
	// key: public key of committee, value: payment address reward receiver
	RewardReceiver map[string]string `json:"RewardReceiver"` // map incognito public key -> reward receiver (payment address)
	// cross shard state for all the shard. from shardID -> to crossShard shardID -> last height
	// e.g 1 -> 2 -> 3 // shard 1 send cross shard to shard 2 at  height 3
	// e.g 1 -> 3 -> 2 // shard 1 send cross shard to shard 3 at  height 2
	LastCrossShardState map[byte]map[byte]uint64 `json:"LastCrossShardState"`
	ShardHandle         map[byte]bool            `json:"ShardHandle"` // lock sync.RWMutex
	// Number of blocks produced by producers in epoch
	NumOfBlocksByProducers map[string]uint64 `json:"NumOfBlocksByProducers"`
	BlockInterval          time.Duration
	BlockMaxCreateTime     time.Duration
	//================================ local state
	currentPDEState *CurrentPDEState
	//================================ StateDB Method
	// block height => root hash
	consensusStateDB         *statedb.StateDB
	ConsensusStateDBRootHash common.Hash
	rewardStateDB            *statedb.StateDB
	RewardStateDBRootHash    common.Hash
	featureStateDB           *statedb.StateDB
	FeatureStateDBRootHash   common.Hash
	slashStateDB             *statedb.StateDB
	SlashStateDBRootHash     common.Hash
}

func (beaconBestState *BeaconBestState) GetBeaconSlashStateDB() *statedb.StateDB {
	return beaconBestState.slashStateDB.Copy()
}

func (beaconBestState *BeaconBestState) GetBeaconFeatureStateDB() *statedb.StateDB {
	return beaconBestState.featureStateDB.Copy()
}

func (beaconBestState *BeaconBestState) GetBeaconRewardStateDB() *statedb.StateDB {
	return beaconBestState.rewardStateDB.Copy()
}

func (beaconBestState *BeaconBestState) GetBeaconConsensusStateDB() *statedb.StateDB {
	return beaconBestState.consensusStateDB.Copy()
}

// var beaconBestState *BeaconBestState

func NewBeaconBestState() *BeaconBestState {
	return &BeaconBestState{}
}
func NewBeaconBestStateWithConfig(netparam *Params) *BeaconBestState {
	beaconBestState := NewBeaconBestState()
	beaconBestState.BestBlockHash.SetBytes(make([]byte, 32))
	beaconBestState.BestBlockHash.SetBytes(make([]byte, 32))
	beaconBestState.BestShardHash = make(map[byte]common.Hash)
	beaconBestState.BestShardHeight = make(map[byte]uint64)
	beaconBestState.BeaconHeight = 0
	beaconBestState.BeaconCommittee = []incognitokey.CommitteePublicKey{}
	beaconBestState.BeaconPendingValidator = []incognitokey.CommitteePublicKey{}
	beaconBestState.CandidateShardWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
	beaconBestState.CandidateBeaconWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
	beaconBestState.CandidateShardWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
	beaconBestState.CandidateBeaconWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
	beaconBestState.RewardReceiver = make(map[string]string)
	beaconBestState.ShardCommittee = make(map[byte][]incognitokey.CommitteePublicKey)
	beaconBestState.ShardPendingValidator = make(map[byte][]incognitokey.CommitteePublicKey)
	beaconBestState.AutoStaking = make(map[string]bool)
	beaconBestState.Params = make(map[string]string)
	beaconBestState.CurrentRandomNumber = -1
	beaconBestState.MaxBeaconCommitteeSize = netparam.MaxBeaconCommitteeSize
	beaconBestState.MinBeaconCommitteeSize = netparam.MinBeaconCommitteeSize
	beaconBestState.MaxShardCommitteeSize = netparam.MaxShardCommitteeSize
	beaconBestState.MinShardCommitteeSize = netparam.MinShardCommitteeSize
	beaconBestState.ActiveShards = netparam.ActiveShards
	beaconBestState.LastCrossShardState = make(map[byte]map[byte]uint64)
	beaconBestState.BlockInterval = netparam.MinBeaconBlockInterval
	beaconBestState.BlockMaxCreateTime = netparam.MaxBeaconBlockCreation
	beaconBestState.currentPDEState = new(CurrentPDEState)
	return beaconBestState
}

func (bc *BlockChain) GetBeaconBestState() *BeaconBestState {
	return bc.BeaconChain.multiView.GetBestView().(*BeaconBestState)
}

func (beaconBestState *BeaconBestState) InitStateRootHashFromDatabase(bc *BlockChain) error {
	db := bc.GetBeaconChainDatabase()
	var dbAccessWarper = statedb.NewDatabaseAccessWarper(db)
	if rootHash, err := bc.GetBeaconConsensusRootHash(db, beaconBestState.BeaconHeight); err == nil {
		beaconBestState.consensusStateDB, err = statedb.NewWithPrefixTrie(rootHash, dbAccessWarper)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	if rootHash, err := bc.GetBeaconFeatureRootHash(db, beaconBestState.BeaconHeight); err == nil {
		beaconBestState.featureStateDB, err = statedb.NewWithPrefixTrie(rootHash, dbAccessWarper)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	if rootHash, err := bc.GetBeaconRewardRootHash(db, beaconBestState.BeaconHeight); err == nil {
		beaconBestState.rewardStateDB, err = statedb.NewWithPrefixTrie(rootHash, dbAccessWarper)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	if rootHash, err := bc.GetBeaconSlashRootHash(db, beaconBestState.BeaconHeight); err == nil {
		beaconBestState.slashStateDB, err = statedb.NewWithPrefixTrie(rootHash, dbAccessWarper)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func (beaconBestState *BeaconBestState) InitStateRootHash(bc *BlockChain) error {
	db := bc.GetBeaconChainDatabase()
	var err error
	var dbAccessWarper = statedb.NewDatabaseAccessWarper(db)
	beaconBestState.consensusStateDB, err = statedb.NewWithPrefixTrie(beaconBestState.ConsensusStateDBRootHash, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.featureStateDB, err = statedb.NewWithPrefixTrie(beaconBestState.FeatureStateDBRootHash, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.rewardStateDB, err = statedb.NewWithPrefixTrie(beaconBestState.RewardStateDBRootHash, dbAccessWarper)
	if err != nil {
		return err
	}
	beaconBestState.slashStateDB, err = statedb.NewWithPrefixTrie(beaconBestState.SlashStateDBRootHash, dbAccessWarper)
	if err != nil {
		return err
	}
	return nil
}

func (beaconBestState *BeaconBestState) MarshalJSON() ([]byte, error) {
	type Alias BeaconBestState
	b, err := json.Marshal(&struct {
		*Alias
	}{
		(*Alias)(beaconBestState),
	})
	if err != nil {
		Logger.log.Error(err)
	}
	return b, err
}

func (beaconBestState *BeaconBestState) GetProducerIndexFromBlock(block *BeaconBlock) int {
	//TODO: revert his
	//return (beaconBestState.BeaconProposerIndex + block.Header.Round) % len(beaconBestState.BeaconCommittee)
	return 0
}

func (beaconBestState *BeaconBestState) SetBestShardHeight(shardID byte, height uint64) {

	beaconBestState.BestShardHeight[shardID] = height
}

func (beaconBestState *BeaconBestState) GetShardConsensusAlgorithm() map[byte]string {

	res := make(map[byte]string)
	for index, element := range beaconBestState.ShardConsensusAlgorithm {
		res[index] = element
	}
	return res
}

func (beaconBestState *BeaconBestState) GetBestShardHash() map[byte]common.Hash {
	res := make(map[byte]common.Hash)
	for index, element := range beaconBestState.BestShardHash {
		res[index] = element
	}
	return res
}

func (beaconBestState *BeaconBestState) GetBestShardHeight() map[byte]uint64 {

	res := make(map[byte]uint64)
	for index, element := range beaconBestState.BestShardHeight {
		res[index] = element
	}
	return res
}

func (beaconBestState *BeaconBestState) GetBestHeightOfShard(shardID byte) uint64 {

	return beaconBestState.BestShardHeight[shardID]
}

func (beaconBestState *BeaconBestState) GetAShardCommittee(shardID byte) []incognitokey.CommitteePublicKey {

	return beaconBestState.ShardCommittee[shardID]
}

func (beaconBestState *BeaconBestState) GetShardCommittee() (res map[byte][]incognitokey.CommitteePublicKey) {

	res = make(map[byte][]incognitokey.CommitteePublicKey)
	for index, element := range beaconBestState.ShardCommittee {
		res[index] = element
	}
	return res
}

func (beaconBestState *BeaconBestState) GetAShardPendingValidator(shardID byte) []incognitokey.CommitteePublicKey {

	return beaconBestState.ShardPendingValidator[shardID]
}

func (beaconBestState *BeaconBestState) GetShardPendingValidator() (res map[byte][]incognitokey.CommitteePublicKey) {

	res = make(map[byte][]incognitokey.CommitteePublicKey)
	for index, element := range beaconBestState.ShardPendingValidator {
		res[index] = element
	}
	return res
}

func (beaconBestState *BeaconBestState) GetCurrentShard() byte {

	for shardID, isCurrent := range beaconBestState.ShardHandle {
		if isCurrent {
			return shardID
		}
	}
	return 0
}

func (beaconBestState *BeaconBestState) SetMaxShardCommitteeSize(maxShardCommitteeSize int) bool {
	// check input params, below MinCommitteeSize failed to acheive consensus
	if maxShardCommitteeSize < MinCommitteeSize {
		return false
	}
	// max committee size can't be lower than current min committee size
	if maxShardCommitteeSize >= beaconBestState.MinShardCommitteeSize {
		beaconBestState.MaxShardCommitteeSize = maxShardCommitteeSize
		return true
	}
	return false
}

func (beaconBestState *BeaconBestState) SetMinShardCommitteeSize(minShardCommitteeSize int) bool {
	// check input params, below MinCommitteeSize failed to acheive consensus
	if minShardCommitteeSize < MinCommitteeSize {
		return false
	}
	// min committee size can't be greater than current min committee size
	if minShardCommitteeSize <= beaconBestState.MaxShardCommitteeSize {
		beaconBestState.MinShardCommitteeSize = minShardCommitteeSize
		return true
	}
	return false
}

func (beaconBestState *BeaconBestState) SetMaxBeaconCommitteeSize(maxBeaconCommitteeSize int) bool {
	// check input params, below MinCommitteeSize failed to acheive consensus
	if maxBeaconCommitteeSize < MinCommitteeSize {
		return false
	}
	// max committee size can't be lower than current min committee size
	if maxBeaconCommitteeSize >= beaconBestState.MinBeaconCommitteeSize {
		beaconBestState.MaxBeaconCommitteeSize = maxBeaconCommitteeSize
		return true
	}
	return false
}

func (beaconBestState *BeaconBestState) SetMinBeaconCommitteeSize(minBeaconCommitteeSize int) bool {
	// check input params, below MinCommitteeSize failed to acheive consensus
	if minBeaconCommitteeSize < MinCommitteeSize {
		return false
	}
	// min committee size can't be greater than current min committee size
	if minBeaconCommitteeSize <= beaconBestState.MaxBeaconCommitteeSize {
		beaconBestState.MinBeaconCommitteeSize = minBeaconCommitteeSize
		return true
	}
	return false
}
func (beaconBestState *BeaconBestState) CheckCommitteeSize() error {
	if beaconBestState.MaxBeaconCommitteeSize < MinCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect max beacon size %+v equal or greater than min size %+v", beaconBestState.MaxBeaconCommitteeSize, MinCommitteeSize))
	}
	if beaconBestState.MinBeaconCommitteeSize < MinCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect min beacon size %+v equal or greater than min size %+v", beaconBestState.MinBeaconCommitteeSize, MinCommitteeSize))
	}
	if beaconBestState.MaxShardCommitteeSize < MinCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect max shard size %+v equal or greater than min size %+v", beaconBestState.MaxShardCommitteeSize, MinCommitteeSize))
	}
	if beaconBestState.MinShardCommitteeSize < MinCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect min shard size %+v equal or greater than min size %+v", beaconBestState.MinShardCommitteeSize, MinCommitteeSize))
	}
	if beaconBestState.MaxBeaconCommitteeSize < beaconBestState.MinBeaconCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect Max beacon size is higher than min beacon size but max is %+v and min is %+v", beaconBestState.MaxBeaconCommitteeSize, beaconBestState.MinBeaconCommitteeSize))
	}
	if beaconBestState.MaxShardCommitteeSize < beaconBestState.MinShardCommitteeSize {
		return NewBlockChainError(CommitteeOrValidatorError, fmt.Errorf("Expect Max beacon size is higher than min beacon size but max is %+v and min is %+v", beaconBestState.MaxBeaconCommitteeSize, beaconBestState.MinBeaconCommitteeSize))
	}
	return nil
}

func (beaconBestState *BeaconBestState) GetBytes() []byte {

	var keys []int
	var keyStrs []string
	res := []byte{}
	res = append(res, beaconBestState.BestBlockHash.GetBytes()...)
	res = append(res, beaconBestState.PreviousBestBlockHash.GetBytes()...)
	res = append(res, beaconBestState.BestBlock.Hash().GetBytes()...)
	res = append(res, beaconBestState.BestBlock.Header.PreviousBlockHash.GetBytes()...)
	for k := range beaconBestState.BestShardHash {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		hash := beaconBestState.BestShardHash[byte(shardID)]
		res = append(res, hash.GetBytes()...)
	}
	keys = []int{}
	for k := range beaconBestState.BestShardHeight {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		height := beaconBestState.BestShardHeight[byte(shardID)]
		res = append(res, byte(height))
	}
	EpochBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(EpochBytes, beaconBestState.Epoch)
	res = append(res, EpochBytes...)
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, beaconBestState.BeaconHeight)
	res = append(res, heightBytes...)
	res = append(res, []byte(strconv.Itoa(beaconBestState.BeaconProposerIndex))...)
	for _, value := range beaconBestState.BeaconCommittee {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	for _, value := range beaconBestState.BeaconPendingValidator {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	for _, value := range beaconBestState.CandidateBeaconWaitingForCurrentRandom {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	for _, value := range beaconBestState.CandidateBeaconWaitingForNextRandom {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	for _, value := range beaconBestState.CandidateShardWaitingForCurrentRandom {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	for _, value := range beaconBestState.CandidateShardWaitingForNextRandom {
		valueBytes, err := value.Bytes()
		if err != nil {
			return nil
		}
		res = append(res, valueBytes...)
	}
	keys = []int{}
	for k := range beaconBestState.ShardCommittee {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		for _, value := range beaconBestState.ShardCommittee[byte(shardID)] {
			valueBytes, err := value.Bytes()
			if err != nil {
				return nil
			}
			res = append(res, valueBytes...)
		}
	}
	keys = []int{}
	for k := range beaconBestState.ShardPendingValidator {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		for _, value := range beaconBestState.ShardPendingValidator[byte(shardID)] {
			valueBytes, err := value.Bytes()
			if err != nil {
				return nil
			}
			res = append(res, valueBytes...)
		}
	}
	keysStrs2 := []string{}
	for k := range beaconBestState.AutoStaking {
		keysStrs2 = append(keysStrs2, k)
	}
	sort.Strings(keysStrs2)
	for _, key := range keysStrs2 {
		if beaconBestState.AutoStaking[key] {
			res = append(res, []byte("true")...)
		} else {
			res = append(res, []byte("false")...)
		}
	}
	randomNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(randomNumBytes, uint64(beaconBestState.CurrentRandomNumber))
	res = append(res, randomNumBytes...)

	randomTimeBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(randomTimeBytes, uint64(beaconBestState.CurrentRandomTimeStamp))
	res = append(res, randomTimeBytes...)

	if beaconBestState.IsGetRandomNumber {
		res = append(res, []byte("true")...)
	} else {
		res = append(res, []byte("false")...)
	}
	for k := range beaconBestState.Params {
		keyStrs = append(keyStrs, k)
	}
	sort.Strings(keyStrs)
	for _, key := range keyStrs {
		res = append(res, []byte(beaconBestState.Params[key])...)
	}

	keys = []int{}
	for k := range beaconBestState.ShardHandle {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, shardID := range keys {
		shardHandleItem := beaconBestState.ShardHandle[byte(shardID)]
		if shardHandleItem {
			res = append(res, []byte("true")...)
		} else {
			res = append(res, []byte("false")...)
		}
	}
	res = append(res, []byte(strconv.Itoa(beaconBestState.MaxBeaconCommitteeSize))...)
	res = append(res, []byte(strconv.Itoa(beaconBestState.MinBeaconCommitteeSize))...)
	res = append(res, []byte(strconv.Itoa(beaconBestState.MaxShardCommitteeSize))...)
	res = append(res, []byte(strconv.Itoa(beaconBestState.MinShardCommitteeSize))...)
	res = append(res, []byte(strconv.Itoa(beaconBestState.ActiveShards))...)

	keys = []int{}
	for k := range beaconBestState.LastCrossShardState {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, fromShard := range keys {
		fromShardMap := beaconBestState.LastCrossShardState[byte(fromShard)]
		newKeys := []int{}
		for k := range fromShardMap {
			newKeys = append(newKeys, int(k))
		}
		sort.Ints(newKeys)
		for _, toShard := range newKeys {
			value := fromShardMap[byte(toShard)]
			valueBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(valueBytes, value)
			res = append(res, valueBytes...)
		}
	}
	return res
}
func (beaconBestState *BeaconBestState) Hash() common.Hash {
	return common.HashH(beaconBestState.GetBytes())
}

func (beaconBestState *BeaconBestState) GetShardCandidate() []incognitokey.CommitteePublicKey {

	return append(beaconBestState.CandidateShardWaitingForCurrentRandom, beaconBestState.CandidateShardWaitingForNextRandom...)
}
func (beaconBestState *BeaconBestState) GetBeaconCandidate() []incognitokey.CommitteePublicKey {

	return append(beaconBestState.CandidateBeaconWaitingForCurrentRandom, beaconBestState.CandidateBeaconWaitingForNextRandom...)
}
func (beaconBestState *BeaconBestState) GetBeaconCommittee() []incognitokey.CommitteePublicKey {

	result := []incognitokey.CommitteePublicKey{}
	return append(result, beaconBestState.BeaconCommittee...)
}

func (beaconBestState *BeaconBestState) GetCommittee() []incognitokey.CommitteePublicKey {

	result := []incognitokey.CommitteePublicKey{}
	return append(result, beaconBestState.BeaconCommittee...)
}

func (beaconBestState *BeaconBestState) GetProposerByTimeSlot(ts int64) incognitokey.CommitteePublicKey {
	id := int(ts) % len(beaconBestState.BeaconCommittee)
	//fmt.Println("debug GetProposerByTimeSlot", len(beaconBestState.BeaconCommittee), int(ts), id)
	return beaconBestState.BeaconCommittee[id]
}

func (beaconBestState *BeaconBestState) GetBlock() common.BlockInterface {
	return &beaconBestState.BestBlock
}

func (beaconBestState *BeaconBestState) GetBeaconPendingValidator() []incognitokey.CommitteePublicKey {

	return beaconBestState.BeaconPendingValidator
}

func (beaconBestState *BeaconBestState) GetRewardReceiver() map[string]string {
	return beaconBestState.RewardReceiver
}

func (beaconBestState *BeaconBestState) GetAutoStaking() map[string]bool {
	return beaconBestState.AutoStaking
}

func (beaconBestState *BeaconBestState) cloneBeaconBestStateFrom(target *BeaconBestState) error {
	tempMarshal, err := target.MarshalJSON()
	if err != nil {
		return NewBlockChainError(MashallJsonBeaconBestStateError, fmt.Errorf("Shard Best State %+v get %+v", beaconBestState.BeaconHeight, err))
	}
	err = json.Unmarshal(tempMarshal, beaconBestState)
	if err != nil {
		return NewBlockChainError(UnmashallJsonBeaconBestStateError, fmt.Errorf("Clone Shard Best State %+v get %+v", beaconBestState.BeaconHeight, err))
	}
	plainBeaconBestState := NewBeaconBestState()
	if reflect.DeepEqual(*beaconBestState, plainBeaconBestState) {
		return NewBlockChainError(CloneBeaconBestStateError, fmt.Errorf("Shard Best State %+v clone failed", beaconBestState.BeaconHeight))
	}
	beaconBestState.consensusStateDB = target.consensusStateDB.Copy()
	beaconBestState.featureStateDB = target.featureStateDB.Copy()
	beaconBestState.rewardStateDB = target.rewardStateDB.Copy()
	beaconBestState.slashStateDB = target.slashStateDB.Copy()
	beaconBestState.currentPDEState = target.currentPDEState.Copy()
	return nil
}

func (beaconBestState *BeaconBestState) CloneBeaconBestStateFrom(target *BeaconBestState) error {
	return beaconBestState.cloneBeaconBestStateFrom(target)
}

func (beaconBestState *BeaconBestState) updateLastCrossShardState(shardStates map[byte][]ShardState) {
	lastCrossShardState := beaconBestState.LastCrossShardState
	for fromShard, shardBlocks := range shardStates {
		for _, shardBlock := range shardBlocks {
			for _, toShard := range shardBlock.CrossShard {
				if fromShard == toShard {
					continue
				}
				if lastCrossShardState[fromShard] == nil {
					lastCrossShardState[fromShard] = make(map[byte]uint64)
				}
				waitHeight := shardBlock.Height
				lastCrossShardState[fromShard][toShard] = waitHeight
			}
		}
	}
}
func (beaconBestState *BeaconBestState) UpdateLastCrossShardState(shardStates map[byte][]ShardState) {
	beaconBestState.updateLastCrossShardState(shardStates)
}

func (beaconBestState *BeaconBestState) GetAutoStakingList() map[string]bool {

	m := make(map[string]bool)
	for k, v := range beaconBestState.AutoStaking {
		m[k] = v
	}
	return m
}

func (beaconBestState *BeaconBestState) GetAllCommitteeValidatorCandidateFlattenList() []string {

	return beaconBestState.getAllCommitteeValidatorCandidateFlattenList()
}

func (beaconBestState *BeaconBestState) getAllCommitteeValidatorCandidateFlattenList() []string {
	res := []string{}
	for _, committee := range beaconBestState.ShardCommittee {
		committeeStr, err := incognitokey.CommitteeKeyListToString(committee)
		if err != nil {
			panic(err)
		}
		res = append(res, committeeStr...)
	}
	for _, pendingValidator := range beaconBestState.ShardPendingValidator {
		pendingValidatorStr, err := incognitokey.CommitteeKeyListToString(pendingValidator)
		if err != nil {
			panic(err)
		}
		res = append(res, pendingValidatorStr...)
	}

	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconCommittee)
	if err != nil {
		panic(err)
	}
	res = append(res, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconPendingValidator)
	if err != nil {
		panic(err)
	}
	res = append(res, beaconPendingValidatorStr...)

	candidateBeaconWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateBeaconWaitingForCurrentRandomStr...)

	candidateBeaconWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateBeaconWaitingForNextRandomStr...)

	candidateShardWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateShardWaitingForCurrentRandomStr...)

	candidateShardWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateShardWaitingForNextRandomStr...)
	return res
}

func (beaconBestState *BeaconBestState) GetHash() *common.Hash {
	return beaconBestState.BestBlock.Hash()
}

func (beaconBestState *BeaconBestState) GetPreviousHash() *common.Hash {
	return &beaconBestState.BestBlock.Header.PreviousBlockHash
}

func (beaconBestState *BeaconBestState) GetHeight() uint64 {
	return beaconBestState.BestBlock.GetHeight()
}

func (beaconBestState *BeaconBestState) GetBlockTime() int64 {
	return beaconBestState.BestBlock.Header.Timestamp
}

func (beaconBestState *BeaconBestState) GetAllCommitteeValidatorCandidate() (map[byte][]incognitokey.CommitteePublicKey, map[byte][]incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, error) {
	SC := make(map[byte][]incognitokey.CommitteePublicKey)
	SPV := make(map[byte][]incognitokey.CommitteePublicKey)
	for shardID, committee := range beaconBestState.GetShardCommittee() {
		SC[shardID] = append([]incognitokey.CommitteePublicKey{}, committee...)
	}
	for shardID, pendingValidator := range beaconBestState.GetShardPendingValidator() {
		SPV[shardID] = append([]incognitokey.CommitteePublicKey{}, pendingValidator...)
	}
	BC := beaconBestState.BeaconCommittee
	BPV := beaconBestState.BeaconPendingValidator
	CBWFCR := beaconBestState.CandidateBeaconWaitingForCurrentRandom
	CBWFNR := beaconBestState.CandidateBeaconWaitingForNextRandom
	CSWFCR := beaconBestState.CandidateShardWaitingForCurrentRandom
	CSWFNR := beaconBestState.CandidateShardWaitingForNextRandom
	return SC, SPV, BC, BPV, CBWFCR, CBWFNR, CSWFCR, CSWFNR, nil
}

func (beaconBestState *BeaconBestState) GetAllCommitteeValidatorCandidateFlattenListFromDatabase() ([]string, error) {
	res := []string{}
	for _, committee := range beaconBestState.GetShardCommittee() {
		committeeStr, err := incognitokey.CommitteeKeyListToString(committee)
		if err != nil {
			return nil, err
		}
		res = append(res, committeeStr...)
	}
	for _, pendingValidator := range beaconBestState.GetShardPendingValidator() {
		pendingValidatorStr, err := incognitokey.CommitteeKeyListToString(pendingValidator)
		if err != nil {
			return nil, err
		}
		res = append(res, pendingValidatorStr...)
	}
	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconCommittee)
	if err != nil {
		return nil, err
	}
	res = append(res, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.BeaconPendingValidator)
	if err != nil {
		return nil, err
	}
	res = append(res, beaconPendingValidatorStr...)

	candidateBeaconWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		return nil, err
	}
	res = append(res, candidateBeaconWaitingForCurrentRandomStr...)

	candidateBeaconWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		return nil, err
	}
	res = append(res, candidateBeaconWaitingForNextRandomStr...)

	candidateShardWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		return nil, err
	}
	res = append(res, candidateShardWaitingForCurrentRandomStr...)

	candidateShardWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconBestState.CandidateShardWaitingForNextRandom)
	if err != nil {
		return nil, err
	}
	res = append(res, candidateShardWaitingForNextRandomStr...)
	return res, nil
}

func (beaconBestState *BeaconBestState) GetAllBridgeTokens() ([]common.Hash, error) {
	bridgeTokenIDs := []common.Hash{}
	allBridgeTokens := []*rawdbv2.BridgeTokenInfo{}
	bridgeStateDB := beaconBestState.GetBeaconFeatureStateDB()
	allBridgeTokensBytes, err := statedb.GetAllBridgeTokens(bridgeStateDB)
	if err != nil {
		return bridgeTokenIDs, err
	}
	err = json.Unmarshal(allBridgeTokensBytes, &allBridgeTokens)
	if err != nil {
		return bridgeTokenIDs, err
	}
	for _, bridgeTokenInfo := range allBridgeTokens {
		bridgeTokenIDs = append(bridgeTokenIDs, *bridgeTokenInfo.TokenID)
	}
	return bridgeTokenIDs, nil
}

func (blockchain *BlockChain) GetBeaconConsensusRootHash(db incdb.Database, height uint64) (common.Hash, error) {
	return rawdbv2.GetBeaconConsensusStateRootHash(db, height)
}

func (blockchain *BlockChain) GetBeaconRewardRootHash(db incdb.Database, height uint64) (common.Hash, error) {
	return rawdbv2.GetBeaconRewardStateRootHash(db, height)
}

func (blockchain *BlockChain) GetBeaconFeatureRootHash(db incdb.Database, height uint64) (common.Hash, error) {
	return rawdbv2.GetBeaconFeatureStateRootHash(db, height)
}

func (blockchain *BlockChain) GetBeaconSlashRootHash(db incdb.Database, height uint64) (common.Hash, error) {
	return rawdbv2.GetBeaconSlashStateRootHash(db, height)
}
