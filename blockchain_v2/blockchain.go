package blockchain_v2

import (
	"errors"
	"sync"

	_ "github.com/incognitochain/incognito-chain/blockchain_v2/params"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type Blockchain struct {
	lock   sync.RWMutex
	chains map[string]ChainViewManager
	config *Config
}

func (bc *Blockchain) GetChain(chainName string) (consensus.ChainViewManagerInterface, error) {
	if chain, ok := bc.chains[chainName]; ok {
		return chain, nil
	}
	return nil, errors.New("Chain not exist")
}

func (bc *Blockchain) GetAllChains() map[string]consensus.ChainViewManagerInterface {
	return bc.chains
}

func (bc *Blockchain) Init(config *Config) {
	// InitNewChainViewManager(name string, chainID int, rootView consensus.ChainViewInterface)
}
