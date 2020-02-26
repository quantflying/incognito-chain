package app

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/beaconblockv2"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type BeaconView struct {
	//field that copy manualy
	bc     *blockchainV2
	Lock   *sync.RWMutex
	Logger common.Logger

	//field that copy automatically and need to update
	Block blockinterface.BeaconBlockInterface

	BeaconCommittee        []incognitokey.CommitteePublicKey
	BeaconPendingValidator []incognitokey.CommitteePublicKey
	BeaconCommitteeHash    common.Hash

	ShardCommittee        map[byte][]incognitokey.CommitteePublicKey
	ShardPendingValidator map[byte][]incognitokey.CommitteePublicKey

	CandidateBeaconWaitingForCurrentRandom []incognitokey.CommitteePublicKey
	CandidateBeaconWaitingForNextRandom    []incognitokey.CommitteePublicKey
	CandidateShardWaitingForCurrentRandom  []incognitokey.CommitteePublicKey
	CandidateShardWaitingForNextRandom     []incognitokey.CommitteePublicKey

	CurrentRandomTimeStamp int64
	IsGettingRandomNumber  bool
	CurrentRandomNumber    int64

	RewardReceiver map[string]string // map incognito public key -> reward receiver (payment address)
	AutoStaking    map[string]bool

	//================================ StateDB Method
	// block height => root hash
	consensusStateDB *statedb.StateDB
	rewardStateDB    *statedb.StateDB
	featureStateDB   *statedb.StateDB
	slashStateDB     *statedb.StateDB
}

func (beaconView *BeaconView) GetAShardCommitee(shardID byte) []incognitokey.CommitteePublicKey {
	beaconView.Lock.RLock()
	defer beaconView.Lock.RUnlock()
	return beaconView.ShardCommittee[shardID]
}

func (beaconView *BeaconView) CreateBlockFromOldBlockData(block blockinterface.BlockInterface) blockinterface.BlockInterface {
	block1 := block.(*beaconblockv2.BeaconBlock)
	block1.ConsensusHeader.TimeSlot = common.GetTimeSlot(beaconView.GetGenesisTime(), time.Now().Unix(), blsbftv2.TIMESLOT)
	return block1
}

func (beaconView *BeaconView) GetBlock() blockinterface.BlockInterface {
	return beaconView.Block
}

func (beaconView *BeaconView) GetCopiedSlashStateDB() *statedb.StateDB {
	return beaconView.slashStateDB.Copy()
}

func (beaconView *BeaconView) GetCopiedFeatureStateDB() *statedb.StateDB {
	return beaconView.featureStateDB.Copy()
}

func (beaconView *BeaconView) GetCopiedRewardStateDB() *statedb.StateDB {
	return beaconView.rewardStateDB.Copy()
}

func (beaconView *BeaconView) GetCopiedConsensusStateDB() *statedb.StateDB {
	return beaconView.consensusStateDB.Copy()
}

func (beaconView *BeaconView) UnmarshalBlock(b []byte) (blockinterface.BlockInterface, error) {
	block, err := UnmarshalBeaconBlock(b)
	if err != nil {
		return nil, err
	}
	return block.(blockinterface.BlockInterface), nil
}

func (beaconView *BeaconView) GetGenesisTime() int64 {
	return beaconView.bc.chainParams.GenesisBeaconBlock.Header.Timestamp
}

func (beaconView *BeaconView) GetConsensusConfig() string {
	panic("implement me")
}

func (beaconView *BeaconView) GetConsensusType() string {
	return "bls"
}

func (beaconView *BeaconView) GetBlkMinInterval() time.Duration {
	return beaconView.bc.chainParams.MinBeaconBlockInterval
}

func (beaconView *BeaconView) GetBlkMaxCreateTime() time.Duration {
	return beaconView.bc.chainParams.MaxBeaconBlockCreation
}

func (beaconView *BeaconView) GetPubkeyRole(pubkey string, timeslot int) (string, byte) {
	panic("implement me")
}

func (beaconView *BeaconView) GetPublicKeyStatus(pubkey string) (status string, isBeacon bool, shardID byte) {
	beaconView.Lock.RLock()
	defer beaconView.Lock.RUnlock()
	for _, key := range beaconView.BeaconCommittee {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_COMMITTEE, true, 0
		}
	}
	for _, key := range beaconView.BeaconPendingValidator {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_PENDING, true, 0
		}
	}

	for _, key := range beaconView.CandidateBeaconWaitingForCurrentRandom {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_WAITING, true, 0
		}
	}
	for _, key := range beaconView.CandidateBeaconWaitingForNextRandom {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_WAITING, true, 0
		}
	}
	for _, key := range beaconView.CandidateShardWaitingForCurrentRandom {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_WAITING, false, 0
		}
	}
	for _, key := range beaconView.CandidateShardWaitingForNextRandom {
		if key.GetIncKeyBase58() == pubkey {
			return common.MININGKEY_STATUS_WAITING, false, 0
		}
	}

	for shardID, shardCommittee := range beaconView.ShardCommittee {
		for _, key := range shardCommittee {
			if key.GetIncKeyBase58() == pubkey {
				return common.MININGKEY_STATUS_COMMITTEE, false, shardID
			}
		}
	}

	for shardID, shardCommittee := range beaconView.ShardPendingValidator {
		for _, key := range shardCommittee {
			if key.GetIncKeyBase58() == pubkey {
				return common.MININGKEY_STATUS_PENDING, false, shardID
			}
		}
	}

	return common.MININGKEY_STATUS_OUTSIDER, false, 0

}

func (beaconView *BeaconView) GetCommittee() []incognitokey.CommitteePublicKey {
	return beaconView.BeaconCommittee
}

func (beaconView BeaconView) GetCommitteeHash() common.Hash {
	return beaconView.BeaconCommitteeHash
}

func (beaconView BeaconView) GetCommitteeIndex(string) int {
	panic("implement me")
}

func (beaconView BeaconView) GetHeight() uint64 {
	return beaconView.Block.GetHeader().GetHeight()
}

// func (s BeaconView) GetRound() int {
// 	return s.Block.GetHeader().GetRound()
// }

func (beaconView BeaconView) GetTimeStamp() int64 {
	return beaconView.Block.GetHeader().GetTimestamp()
}

func (beaconView BeaconView) GetTimeslot() uint64 {
	if beaconView.Block.GetHeader().GetVersion() == 1 {
		return 1
	}
	return beaconView.Block.GetHeader().(blockinterface.BlockHeaderV2Interface).GetTimeslot()
}

func (beaconView BeaconView) GetEpoch() uint64 {
	return beaconView.Block.GetHeader().GetEpoch()
}

func (beaconView BeaconView) Hash() common.Hash {
	return *beaconView.Block.GetHeader().GetHash()
}

func (beaconView BeaconView) GetPreviousViewHash() common.Hash {
	prevHash := beaconView.Block.GetHeader().GetPreviousBlockHash()
	return prevHash
}

func (beaconView BeaconView) GetNextProposer(timeSlot uint64) string {
	committee := beaconView.GetCommittee()
	idx := int(timeSlot) % len(committee)
	return committee[idx].GetMiningKeyBase58(common.BlsConsensus)
}

func (beaconView *BeaconView) CloneNewView() consensus.ChainViewInterface {
	b, _ := beaconView.MarshalJSON()
	var newView *BeaconView
	err := json.Unmarshal(b, &newView)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	newView.Logger = beaconView.Logger
	newView.Lock = &sync.RWMutex{}
	newView.bc = beaconView.bc
	return newView
}

func (beaconView *BeaconView) MarshalJSON() ([]byte, error) {
	type Alias BeaconView
	b, err := json.Marshal(&struct {
		*Alias
		DB     interface{}
		Lock   interface{}
		Logger interface{}
		BC     interface{}
		Block  interface{}
	}{
		(*Alias)(beaconView),
		nil,
		nil,
		nil,
		nil,
		nil,
	})
	if err != nil {
		Logger.log.Error(err)
	}
	return b, err
}

func (beaconView *BeaconView) GetRootTimeSlot() uint64 {
	if beaconView.bc.chainParams.GenesisBeaconBlock.Header.Version == 1 {
		return 1
	}
	return beaconView.Block.GetHeader().(blockinterface.BlockHeaderV2Interface).GetTimeslot()
}

func (beaconView *BeaconView) InitStateRootHash(bc *BlockChain) error {
	panic("implement me")
}
