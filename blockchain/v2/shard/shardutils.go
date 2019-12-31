package shard

import (
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/transaction"
	"time"
)

const GENESIS_TIMESTAMP = "2006-01-02T15:04:05.000Z"

func CreateShardGenesisBlock(
	version int,
	net uint16,
	genesisBlockTime string,
	initTx []string,
) *ShardBlock {
	body := ShardBody{}
	genesisTime, err := time.Parse(GENESIS_TIMESTAMP, genesisBlockTime)
	if err != nil {
		fmt.Println(err)
	}
	header := ShardHeader{
		Timestamp:         genesisTime.Unix(),
		Version:           version,
		BeaconHeight:      1,
		Epoch:             1,
		Round:             1,
		Height:            1,
		PreviousBlockHash: common.Hash{},
	}

	for _, tx := range initTx {
		testSalaryTX := transaction.Tx{}
		testSalaryTX.UnmarshalJSON([]byte(tx))
		body.Transactions = append(body.Transactions, &testSalaryTX)
	}

	block := &ShardBlock{
		Body:   body,
		Header: header,
		ConsensusHeader: ConsensusHeader{
			TimeSlot: common.GetTimeSlot(genesisTime.Unix(), time.Now().Unix(), blsbftv2.TIMESLOT),
			Proposer: "",
		},
	}

	return block
}
