package main

import (
	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type FakeDB struct {
	genesisBlock consensus.BlockInterface
}

func (s *FakeDB) GetRewardOfShardByEpoch(epoch uint64, shardID byte, tokenID common.Hash) (uint64, error) {
	//TODO
	return 0, nil
}

func (s *FakeDB) GetAllTokenIDForReward(epoch uint64) ([]common.Hash, error) {
	//TODO
	return []common.Hash{}, nil
}

func (s *FakeDB) GetGenesisBlock() consensus.BlockInterface {
	return s.genesisBlock
}
