package app

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/shardblockv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type ShardView struct {
	//field that copy manualy
	bc     *blockchainV2
	Lock   *sync.RWMutex
	Logger common.Logger

	//field that copy automatically and need to update
	ShardID               byte
	Block                 blockinterface.ShardBlockInterface
	ShardCommittee        []incognitokey.CommitteePublicKey
	ShardPendingValidator []incognitokey.CommitteePublicKey

	StakingTx map[string]string

	//================================ StateDB Method
	// block height => root hash
	consensusStateDB   *statedb.StateDB
	transactionStateDB *statedb.StateDB
	featureStateDB     *statedb.StateDB
	rewardStateDB      *statedb.StateDB
	slashStateDB       *statedb.StateDB
}

func (shardView *ShardView) GetCopiedSlashStateDB() *statedb.StateDB {
	return shardView.slashStateDB.Copy()
}

func (shardView *ShardView) GetCopiedFeatureStateDB() *statedb.StateDB {
	return shardView.featureStateDB.Copy()
}

func (shardView *ShardView) GetCopiedRewardStateDB() *statedb.StateDB {
	return shardView.rewardStateDB.Copy()
}

func (shardView *ShardView) GetCopiedConsensusStateDB() *statedb.StateDB {
	return shardView.consensusStateDB.Copy()
}

func (shardView *ShardView) GetCopiedTransactionStateDB() *statedb.StateDB {
	return shardView.consensusStateDB.Copy()
}

func (shardView *ShardView) CreateBlockFromOldBlockData(block blockinterface.BlockInterface) blockinterface.BlockInterface {
	block1 := block.(*shardblockv2.ShardBlock)
	block1.ConsensusHeader.TimeSlot = common.GetTimeSlot(shardView.GetGenesisTime(), time.Now().Unix(), blsbftv2.TIMESLOT)
	return block1
}

func (shardView *ShardView) GetBlock() blockinterface.BlockInterface {
	return shardView.Block
}

func (shardView *ShardView) CreateNewViewFromBlock(block blockinterface.BlockInterface) (consensus.ChainViewInterface, error) {
	panic("implement me")
}

func (shardView *ShardView) UnmarshalBlock(b []byte) (blockinterface.BlockInterface, error) {
	block, err := UnmarshalShardBlock(b)
	if err != nil {
		return nil, err
	}
	return block.(blockinterface.BlockInterface), nil
}

func (shardView *ShardView) GetGenesisTime() int64 {
	return shardView.bc.chainParams.GenesisShardBlock.Header.Timestamp
}

func (shardView *ShardView) GetConsensusConfig() string {
	panic("implement me")
}

func (shardView *ShardView) GetConsensusType() string {
	return "bls"
}

func (shardView *ShardView) GetBlkMinInterval() time.Duration {
	panic("implement me")
}

func (shardView *ShardView) GetBlkMaxCreateTime() time.Duration {
	panic("implement me")
}

func (shardView *ShardView) GetPubkeyRole(pubkey string, round int) (string, byte) {
	panic("implement me")
}

func (shardView *ShardView) GetCommittee() []incognitokey.CommitteePublicKey {
	return shardView.ShardCommittee
}

func (shardView *ShardView) GetCommitteeHash() common.Hash {
	panic("implement me")
}

func (shardView *ShardView) GetCommitteeIndex(string) int {
	panic("implement me")
}

func (shardView *ShardView) GetHeight() uint64 {
	return shardView.Block.GetHeader().GetHeight()
}

// func (s *ShardView) GetRound() int {
// 	return s.Block.Header.Round
// }

func (shardView *ShardView) GetTimeStamp() int64 {
	return shardView.Block.GetHeader().GetTimestamp()
}

func (shardView *ShardView) GetTimeslot() uint64 {
	if shardView.Block.GetHeader().GetVersion() == 1 {
		return 1
	}
	return shardView.Block.GetHeader().(blockinterface.BlockHeaderV2Interface).GetTimeslot()
}

func (shardView *ShardView) GetEpoch() uint64 {
	return shardView.Block.GetHeader().GetEpoch()
}

func (shardView *ShardView) Hash() common.Hash {
	return *shardView.Block.GetHeader().GetHash()
}

func (shardView *ShardView) GetPreviousViewHash() common.Hash {
	prevHash := shardView.Block.GetHeader().GetPreviousBlockHash()
	return prevHash
}

func (shardView *ShardView) GetActiveShardNumber() int {
	panic("implement me")
}

func (shardView *ShardView) GetNextProposer(timeSlot uint64) string {
	committee := shardView.GetCommittee()
	idx := int(timeSlot) % len(committee)
	return committee[idx].GetMiningKeyBase58(common.BlsConsensus)
}

func (shardView *ShardView) CloneNewView() consensus.ChainViewInterface {
	b, _ := shardView.MarshalJSON()
	var newView *ShardView
	err := json.Unmarshal(b, &newView)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	newView.Logger = shardView.Logger
	newView.Lock = &sync.RWMutex{}
	newView.bc = shardView.bc
	return newView
}

func (shardView *ShardView) MarshalJSON() ([]byte, error) {
	type Alias ShardView
	b, err := json.Marshal(&struct {
		*Alias
		DB     interface{}
		Lock   interface{}
		Logger interface{}
		BC     interface{}
		Block  interface{}
	}{
		(*Alias)(shardView),
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

func (shardView *ShardView) GetRootTimeSlot() uint64 {
	if shardView.bc.chainParams.GenesisShardBlock.Header.Version == 1 {
		return 1
	}
	return shardView.Block.(blockinterface.BlockHeaderV2Interface).GetTimeslot()
}

func (shardView *ShardView) InitStateRootHash(bc *BlockChain) error {
	panic("implement me")
}
