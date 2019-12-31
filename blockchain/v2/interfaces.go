package v2

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type DB interface {
	GetGenesisBlock() consensus.BlockInterface
	//GetBeaconBlockHashByIndex(uint64) (common.Hash, error)
	//FetchBeaconBlock(common.Hash) ([]byte, error)
}

type BlockChain interface {
	//GetDB() DB
	GetCurrentBeaconHeight() (uint64, error) //get final confirm beacon block height
	GetCurrentEpoch() (uint64, error)        //get final confirm beacon block height
	GetChainParams() blockchain.Params
}

type FakeBC struct {
}

func (FakeBC) GetCurrentBeaconHeight() (uint64, error) {
	return 1, nil
}

func (FakeBC) GetCurrentEpoch() (uint64, error) {
	panic("implement me")
}

func (FakeBC) GetChainParams() blockchain.Params {
	return blockchain.Params{
		Epoch: 10,
	}
}
