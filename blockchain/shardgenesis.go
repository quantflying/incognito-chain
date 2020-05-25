package blockchain

import (
	"fmt"
	"strconv"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/transaction"
)

func CreateShardGenesisBlock(
	version int,
	net uint16,
	genesisBlockTime string,
	icoParams GenesisParams,
) *ShardBlock {
	body := ShardBody{}
	layout := "2006-01-02T15:04:05.000Z"
	str := genesisBlockTime
	genesisTime, err := time.Parse(layout, str)
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

	for _, tx := range icoParams.InitialIncognito {
		testSalaryTX := transaction.Tx{}
		testSalaryTX.UnmarshalJSON([]byte(tx))
		body.Transactions = append(body.Transactions, &testSalaryTX)
	}

	block := &ShardBlock{
		Body:   body,
		Header: header,
	}

	return block
}

func GetShardSwapInstructionKeyListV2(genesisParams GenesisParams) map[byte][][]string {
	allShardSwapInstructionKeyListV2 := make(map[byte][][]string)
	selectShardNodeSerializedPubkeyV2 := genesisParams.SelectShardNodeSerializedPubkeyV2
	preSelectShardNodeSerializedPubkey := genesisParams.PreSelectShardNodeSerializedPubkey
	shardCommitteeSize := MainNetMinShardCommitteeSize
	for i := 0; i < MainNetActiveShards; i++ {
		shardID := byte(i)
		shardSwapInstructionKeyListV2 := [][]string{}
		newCommittees := selectShardNodeSerializedPubkeyV2[:shardCommitteeSize]
		oldCommittees := preSelectShardNodeSerializedPubkey[:shardCommitteeSize]
		for i := 0; i < shardCommitteeSize; i++ {
			newCommittee := newCommittees[i]
			oldCommittee := oldCommittees[i]
			swapInstruction := []string{SwapAction, newCommittee, oldCommittee, "shard", strconv.Itoa(i)}
			shardSwapInstructionKeyListV2 = append(shardSwapInstructionKeyListV2, swapInstruction)
		}
		selectShardNodeSerializedPubkeyV2 = selectShardNodeSerializedPubkeyV2[shardCommitteeSize:]
		preSelectShardNodeSerializedPubkey = preSelectShardNodeSerializedPubkey[shardCommitteeSize:]
		allShardSwapInstructionKeyListV2[shardID] = shardSwapInstructionKeyListV2
	}
	return allShardSwapInstructionKeyListV2
}
