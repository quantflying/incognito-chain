package blockchain_v2

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	_ "github.com/incognitochain/incognito-chain/blockchain_v2/params"
)

type Blockchain struct {
	lock   sync.RWMutex
	chains map[string]ChainViewManager
	config *Config
}

func (bc *Blockchain) GetChain(chainName string, desiredType reflect.Type) (interface{}, error) {
	if !reflect.TypeOf(chainViewManagerInterface).AssignableTo(desiredType) {
		return nil, NewBlockChainError(InterfaceNotCompatibleErr, fmt.Errorf("Returned Type isn't compatible with %v", desiredType.String()))
	}
	if chain, ok := bc.chains[chainName]; ok {
		return chain, nil
	}
	return nil, errors.New("Chain not exist")
}

func (bc *Blockchain) GetAllChains(desiredType reflect.Type) map[string]interface{} {
	if !reflect.TypeOf(chainViewManagerInterface).AssignableTo(desiredType) {
		return nil, NewBlockChainError(InterfaceNotCompatibleErr, fmt.Errorf("Returned Type isn't compatible with %v", desiredType.String()))
	}
	return bc.chains
}

func (bc *Blockchain) Init(config *Config) {
	// InitNewChainViewManager(name string, chainID int, rootView consensus.ChainViewInterface)
}
