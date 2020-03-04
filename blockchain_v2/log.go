package blockchain_v2

import "github.com/incognitochain/incognito-chain/common"

type blockchainLogger struct {
	log common.Logger
}

func (blockchainLogger *blockchainLogger) Init(inst common.Logger) {
	blockchainLogger.log = inst
}

// Global instant to use
var Logger = blockchainLogger{}
