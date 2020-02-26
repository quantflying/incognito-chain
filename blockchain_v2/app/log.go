package app

import "github.com/incognitochain/incognito-chain/common"

type AppLogger struct {
	log common.Logger
}

func (self *AppLogger) Init(inst common.Logger) {
	self.log = inst
}

// Global instant to use
var Logger = AppLogger{}
