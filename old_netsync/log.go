package netsync

import "github.com/incognitochain/incognito-chain/common"

type NetSyncLogger struct {
	log common.Logger
}

func (netSyncLogger *NetSyncLogger) Init(inst common.Logger) {
	netSyncLogger.log = inst
}

// Global instant to use
var Logger = NetSyncLogger{}
