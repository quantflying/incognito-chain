package jsonresult

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type GetBeaconBestState struct {
	BestBlockHash                          common.Hash              `json:"BestBlockHash"`         // The hash of the block.
	PreviousBestBlockHash                  common.Hash              `json:"PreviousBestBlockHash"` // The hash of the block.
	BestShardHash                          map[byte]common.Hash     `json:"BestShardHash"`
	BestShardHeight                        map[byte]uint64          `json:"BestShardHeight"`
	Epoch                                  uint64                   `json:"Epoch"`
	BeaconHeight                           uint64                   `json:"BeaconHeight"`
	BeaconProposerIndex                    int                      `json:"BeaconProposerIndex"`
	BeaconCommittee                        []string                 `json:"BeaconCommittee"`
	BeaconPendingValidator                 []string                 `json:"BeaconPendingValidator"`
	CandidateShardWaitingForCurrentRandom  []string                 `json:"CandidateShardWaitingForCurrentRandom"` // snapshot shard candidate list, waiting to be shuffled in this current epoch
	CandidateBeaconWaitingForCurrentRandom []string                 `json:"CandidateBeaconWaitingForCurrentRandom"`
	CandidateShardWaitingForNextRandom     []string                 `json:"CandidateShardWaitingForNextRandom"` // shard candidate list, waiting to be shuffled in next epoch
	CandidateBeaconWaitingForNextRandom    []string                 `json:"CandidateBeaconWaitingForNextRandom"`
	RewardReceiver                         map[string]string        `json:"RewardReceiver"`        // key: incognito public key of committee, value: payment address reward receiver
	ShardCommittee                         map[byte][]string        `json:"ShardCommittee"`        // current committee and validator of all shard
	ShardPendingValidator                  map[byte][]string        `json:"ShardPendingValidator"` // pending candidate waiting for swap to get in committee of all shard
	AutoStaking                            map[string]bool          `json:"AutoStaking"`
	CurrentRandomNumber                    int64                    `json:"CurrentRandomNumber"`
	CurrentRandomTimeStamp                 int64                    `json:"CurrentRandomTimeStamp"` // random timestamp for this epoch
	IsGetRandomNumber                      bool                     `json:"IsGetRandomNumber"`
	MaxBeaconCommitteeSize                 int                      `json:"MaxBeaconCommitteeSize"`
	MinBeaconCommitteeSize                 int                      `json:"MinBeaconCommitteeSize"`
	MaxShardCommitteeSize                  int                      `json:"MaxShardCommitteeSize"`
	MinShardCommitteeSize                  int                      `json:"MinShardCommitteeSize"`
	ActiveShards                           int                      `json:"ActiveShards"`
	LastCrossShardState                    map[byte]map[byte]uint64 `json:"LastCrossShardState"`
	ShardHandle                            map[byte]bool            `json:"ShardHandle"` // lock sync.RWMutex
}

func NewGetBeaconBestState(data *blockchain.BeaconBestState) *GetBeaconBestState {
	result := &GetBeaconBestState{
		BestBlockHash:          data.BestBlockHash,
		PreviousBestBlockHash:  data.PreviousBestBlockHash,
		Epoch:                  data.Epoch,
		BeaconHeight:           data.BeaconHeight,
		BeaconProposerIndex:    data.BeaconProposerIndex,
		CurrentRandomNumber:    data.CurrentRandomNumber,
		CurrentRandomTimeStamp: data.CurrentRandomTimeStamp,
		IsGetRandomNumber:      data.IsGetRandomNumber,
		MaxShardCommitteeSize:  data.MaxShardCommitteeSize,
		MinShardCommitteeSize:  data.MinShardCommitteeSize,
		MaxBeaconCommitteeSize: data.MaxBeaconCommitteeSize,
		MinBeaconCommitteeSize: data.MinBeaconCommitteeSize,
		ActiveShards:           data.ActiveShards,
	}
	result.BestShardHash = make(map[byte]common.Hash)
	for k, v := range data.BestShardHash {
		result.BestShardHash[k] = v
	}

	result.BestShardHeight = make(map[byte]uint64)
	for k, v := range data.BestShardHeight {
		result.BestShardHeight[k] = v
	}
	tempBeaconCommittee, err := incognitokey.CommitteeKeyListToString(data.BeaconCommittee)
	if err != nil {
		panic(err)
	}
	result.BeaconCommittee = make([]string, len(data.BeaconCommittee))
	copy(result.BeaconCommittee, tempBeaconCommittee)

	tempBeaconPendingValidator, err := incognitokey.CommitteeKeyListToString(data.BeaconPendingValidator)
	if err != nil {
		panic(err)
	}
	result.BeaconPendingValidator = make([]string, len(data.BeaconPendingValidator))
	copy(result.BeaconPendingValidator, tempBeaconPendingValidator)

	tempCandidateShardWaitingForCurrentRandom, err := incognitokey.CommitteeKeyListToString(data.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	result.CandidateShardWaitingForCurrentRandom = make([]string, len(data.CandidateShardWaitingForCurrentRandom))
	copy(result.CandidateShardWaitingForCurrentRandom, tempCandidateShardWaitingForCurrentRandom)

	tempCandidateBeaconWaitingForCurrentRandom, err := incognitokey.CommitteeKeyListToString(data.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	result.CandidateBeaconWaitingForCurrentRandom = make([]string, len(data.CandidateBeaconWaitingForCurrentRandom))
	copy(result.CandidateBeaconWaitingForCurrentRandom, tempCandidateBeaconWaitingForCurrentRandom)

	tempCandidateShardWaitingForNextRandom, err := incognitokey.CommitteeKeyListToString(data.CandidateShardWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	result.CandidateShardWaitingForNextRandom = make([]string, len(data.CandidateShardWaitingForNextRandom))
	copy(result.CandidateShardWaitingForNextRandom, tempCandidateShardWaitingForNextRandom)

	tempCandidateBeaconWaitingForNextRandom, err := incognitokey.CommitteeKeyListToString(data.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	result.CandidateBeaconWaitingForNextRandom = make([]string, len(data.CandidateBeaconWaitingForNextRandom))
	copy(result.CandidateBeaconWaitingForNextRandom, tempCandidateBeaconWaitingForNextRandom)

	result.RewardReceiver = make(map[string]string)
	for k, v := range data.RewardReceiver {
		result.RewardReceiver[k] = v
	}

	result.ShardCommittee = make(map[byte][]string)
	for k, v := range data.ShardCommittee {
		result.ShardCommittee[k] = make([]string, len(v))
		tempV, err := incognitokey.CommitteeKeyListToString(v)
		if err != nil {
			panic(err)
		}
		copy(result.ShardCommittee[k], tempV)
	}

	result.ShardPendingValidator = make(map[byte][]string)
	for k, v := range data.ShardPendingValidator {
		result.ShardPendingValidator[k] = make([]string, len(v))
		tempV, err := incognitokey.CommitteeKeyListToString(v)
		if err != nil {
			panic(err)
		}
		copy(result.ShardPendingValidator[k], tempV)
	}

	result.LastCrossShardState = make(map[byte]map[byte]uint64)
	for k1, v1 := range data.LastCrossShardState {
		result.LastCrossShardState[k1] = make(map[byte]uint64)
		for k2, v2 := range v1 {
			result.LastCrossShardState[k1][k2] = v2
		}
	}
	result.AutoStaking = make(map[string]bool)
	for k, v := range data.AutoStaking {
		result.AutoStaking[k] = v
	}
	return result
}
