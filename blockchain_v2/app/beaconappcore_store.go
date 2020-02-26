package app

import (
	"encoding/json"
	"strings"

	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func storeForSlashing(slashStateDB *statedb.StateDB, beaconBlock blockinterface.BeaconBlockInterface, bc *blockchainV2) error {
	var err error
	punishedProducersFinished := []string{}
	fliterPunishedProducersFinished := []string{}
	beaconHeight := beaconBlock.GetHeader().GetHeight()
	producersBlackList := statedb.GetProducersBlackList(slashStateDB, beaconHeight-1)
	chainParamEpoch := bc.chainParams.Epoch
	newBeaconHeight := beaconBlock.GetHeader().GetHeight()
	if newBeaconHeight%uint64(chainParamEpoch) == 0 { // end of epoch
		for producer := range producersBlackList {
			producersBlackList[producer]--
			if producersBlackList[producer] == 0 {
				punishedProducersFinished = append(punishedProducersFinished, producer)
			}
		}
		for _, producer := range punishedProducersFinished {
			delete(producersBlackList, producer)
		}
	}

	for _, inst := range beaconBlock.GetBody().GetInstructions() {
		if len(inst) == 0 {
			continue
		}
		if inst[0] != SwapAction {
			continue
		}
		badProducersWithPunishmentBytes := []byte{}
		if len(inst) == 6 && inst[3] == "shard" {
			badProducersWithPunishmentBytes = []byte(inst[5])
		}
		if len(inst) == 5 && inst[3] == "beacon" {
			badProducersWithPunishmentBytes = []byte(inst[4])
		}
		if len(badProducersWithPunishmentBytes) == 0 {
			continue
		}

		var badProducersWithPunishment map[string]uint8
		err = json.Unmarshal(badProducersWithPunishmentBytes, &badProducersWithPunishment)
		if err != nil {
			return err
		}
		for producer, punishedEpoches := range badProducersWithPunishment {
			epoches, found := producersBlackList[producer]
			if !found || epoches < punishedEpoches {
				producersBlackList[producer] = punishedEpoches
			}
		}
	}
	for _, punishedProducerFinished := range punishedProducersFinished {
		flag := false
		for producerBlaskList, _ := range producersBlackList {
			if strings.Compare(producerBlaskList, punishedProducerFinished) == 0 {
				flag = true
				break
			}
		}
		if !flag {
			fliterPunishedProducersFinished = append(fliterPunishedProducersFinished, punishedProducerFinished)
		}
	}
	statedb.RemoveProducerBlackList(slashStateDB, fliterPunishedProducersFinished)
	err = statedb.StoreProducersBlackList(slashStateDB, beaconHeight, producersBlackList)
	return err
}
