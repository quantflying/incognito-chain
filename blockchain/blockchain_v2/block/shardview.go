package block

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"sync"
	"time"
)

type ShardView struct {
	//field that copy manualy
	BC     BlockChain
	DB     DB `json:"-"`
	Lock   *sync.RWMutex
	Logger common.Logger

	//field that copy automatically and need to update
	ShardID               byte
	Block                 *ShardBlock
	ShardCommittee        []incognitokey.CommitteePublicKey
	ShardPendingValidator []incognitokey.CommitteePublicKey
}

func (s *ShardView) CreateBlockFromOldBlockData(block consensus.BlockInterface) consensus.BlockInterface {
	block1 := block.(*ShardBlock)
	block1.ConsensusHeader.TimeSlot = common.GetTimeSlot(s.GetGenesisTime(), time.Now().Unix(), blsbftv2.TIMESLOT)
	return block1
}

func (s *ShardView) GetBlock() consensus.BlockInterface {
	return s.Block
}

func (s *ShardView) UpdateViewWithBlock(block consensus.BlockInterface) error {
	panic("implement me")
}

func (s *ShardView) UnmarshalBlock(b []byte) (consensus.BlockInterface, error) {
	var shardBlk *ShardBlock
	err := json.Unmarshal(b, &shardBlk)
	if err != nil {
		return nil, err
	}
	return shardBlk, nil
}

func (s *ShardView) GetGenesisTime() int64 {
	return s.DB.GetGenesisBlock().GetBlockTimestamp()
}

func (s *ShardView) GetConsensusConfig() string {
	panic("implement me")
}

func (s *ShardView) GetConsensusType() string {
	return "bls"
}

func (s *ShardView) GetBlkMinInterval() time.Duration {
	panic("implement me")
}

func (s *ShardView) GetBlkMaxCreateTime() time.Duration {
	panic("implement me")
}

func (s *ShardView) GetPubkeyRole(pubkey string, round int) (string, byte) {
	panic("implement me")
}

func (s *ShardView) GetCommittee() []incognitokey.CommitteePublicKey {
	return s.ShardCommittee
}

func (s *ShardView) GetCommitteeHash() *common.Hash {
	panic("implement me")
}

func (s *ShardView) GetCommitteeIndex(string) int {
	panic("implement me")
}

func (s *ShardView) GetHeight() uint64 {
	return s.Block.Header.Height
}

func (s *ShardView) GetRound() int {
	return s.Block.Header.Round
}

func (s *ShardView) GetTimeStamp() int64 {
	return s.Block.GetBlockTimestamp()
}

func (s *ShardView) GetTimeslot() uint64 {
	return s.Block.ConsensusHeader.TimeSlot
}

func (s *ShardView) GetEpoch() uint64 {
	return s.Block.GetCurrentEpoch()
}

func (s *ShardView) Hash() common.Hash {
	return *s.Block.Hash()
}

func (s *ShardView) GetPreviousViewHash() *common.Hash {
	prevHash := s.Block.GetPreviousBlockHash()
	return &prevHash
}

func (s *ShardView) GetActiveShardNumber() int {
	panic("implement me")
}

func (s *ShardView) IsBestView() bool {
	panic("implement me")
}

func (s *ShardView) SetViewIsBest(isBest bool) {
	panic("implement me")
}

func (s *ShardView) DeleteView() error {
	panic("implement me")
}

func (s *ShardView) CloneViewFrom(view consensus.ChainViewInterface) error {
	panic("implement me")
}

func (s *ShardView) GetNextProposer(timeSlot uint64) string {
	committee := s.GetCommittee()
	idx := int(timeSlot) % len(committee)
	return committee[idx].GetMiningKeyBase58(common.BlsConsensus2)
}

func (s *ShardView) CloneNewView() consensus.ChainViewInterface {
	b, _ := s.MarshalJSON()
	var newView *ShardView
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

func (s *ShardView) MarshalJSON() ([]byte, error) {
	type Alias ShardView
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

func (s *ShardView) GetRootTimeSlot() uint64 {
	return s.DB.GetGenesisBlock().GetTimeslot()
}
