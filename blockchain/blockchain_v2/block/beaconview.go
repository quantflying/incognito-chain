package block

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type BeaconView struct {
	//field that copy manualy
	BC     BlockChain
	DB     DB `json:"-"`
	Lock   *sync.RWMutex
	Logger common.Logger

	//field that copy automatically and need to update
	Block *BeaconBlock

	BeaconCommittee        []incognitokey.CommitteePublicKey
	BeaconPendingValidator []incognitokey.CommitteePublicKey

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
}

func (s *BeaconView) GetAShardCommitee(shardID byte) []incognitokey.CommitteePublicKey {
	s.Lock.RLock()
	defer s.Lock.RUnlock()
	return s.ShardCommittee[shardID]
}

func (s *BeaconView) GetActiveShard() int {
	//TODO
	return 4
}

func (s *BeaconView) CreateBlockFromOldBlockData(block consensus.BlockInterface) consensus.BlockInterface {
	block1 := block.(*BeaconBlock)
	block1.ConsensusHeader.TimeSlot = common.GetTimeSlot(s.GetGenesisTime(), time.Now().Unix(), blsbftv2.TIMESLOT)
	return block1
}

func (s *BeaconView) GetBlock() consensus.BlockInterface {
	return s.Block
}

func (s *BeaconView) CreateNewViewFromBlock(block consensus.BlockInterface) (consensus.ChainViewInterface, error) {
	panic("implement me")
}

func (s *BeaconView) UnmarshalBlock(b []byte) (consensus.BlockInterface, error) {
	var beaconBlk *BeaconBlock
	err := json.Unmarshal(b, &beaconBlk)
	if err != nil {
		return nil, err
	}
	return beaconBlk, nil
}

func (s *BeaconView) GetGenesisTime() int64 {
	return s.DB.GetGenesisBlock().GetBlockTimestamp()
}

func (s *BeaconView) GetConsensusConfig() string {
	panic("implement me")
}

func (s *BeaconView) GetConsensusType() string {
	return "bls"
}

func (s *BeaconView) GetBlkMinInterval() time.Duration {
	panic("implement me")
}

func (s *BeaconView) GetBlkMaxCreateTime() time.Duration {
	panic("implement me")
}

func (s *BeaconView) GetPubkeyRole(pubkey string, round int) (string, byte) {
	panic("implement me")
}

func (s *BeaconView) GetCommittee() []incognitokey.CommitteePublicKey {
	return s.BeaconCommittee
}

func (s *BeaconView) GetCommitteeHash() *common.Hash {
	panic("implement me")
}

func (s *BeaconView) GetCommitteeIndex(string) int {
	panic("implement me")
}

func (s *BeaconView) GetHeight() uint64 {
	return s.Block.Header.Height
}

func (s *BeaconView) GetRound() int {
	return s.Block.Header.Round
}

func (s *BeaconView) GetTimeStamp() int64 {
	return s.Block.GetBlockTimestamp()
}

func (s *BeaconView) GetTimeslot() uint64 {
	return s.Block.ConsensusHeader.TimeSlot
}

func (s *BeaconView) GetEpoch() uint64 {
	return s.Block.GetCurrentEpoch()
}

func (s *BeaconView) Hash() common.Hash {
	return *s.Block.Hash()
}

func (s *BeaconView) GetPreviousViewHash() *common.Hash {
	prevHash := s.Block.GetPreviousBlockHash()
	return &prevHash
}

func (s *BeaconView) GetActiveShardNumber() int {
	panic("implement me")
}

func (s *BeaconView) IsBestView() bool {
	panic("implement me")
}

func (s *BeaconView) SetViewIsBest(isBest bool) {
	panic("implement me")
}

func (s *BeaconView) DeleteView() error {
	panic("implement me")
}

func (s *BeaconView) CloneViewFrom(view consensus.ChainViewInterface) error {
	panic("implement me")
}

func (s *BeaconView) GetNextProposer(timeSlot uint64) string {
	committee := s.GetCommittee()
	idx := int(timeSlot) % len(committee)
	return committee[idx].GetMiningKeyBase58(common.BlsConsensus2)
}

func (s *BeaconView) CloneNewView() consensus.ChainViewInterface {
	b, _ := s.MarshalJSON()
	var newView *BeaconView
	err := json.Unmarshal(b, &newView)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	newView.DB = s.DB
	newView.Logger = s.Logger
	newView.Lock = &sync.RWMutex{}
	newView.BC = s.BC
	return newView
}

func (s *BeaconView) MarshalJSON() ([]byte, error) {
	type Alias BeaconView
	b, err := json.Marshal(&struct {
		*Alias
		DB     interface{}
		Lock   interface{}
		Logger interface{}
		BC     interface{}
	}{
		(*Alias)(s),
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

func (s *BeaconView) GetRootTimeSlot() uint64 {
	return s.DB.GetGenesisBlock().GetTimeslot()
}
