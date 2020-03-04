package main

import "sync"

type nodeState struct {
	lock         sync.RWMutex
	enableMining bool
	user         struct {
		miningKeys string
		privateKey string
	}
	chains map[string]struct {
	}
	nodeMode string
}

func (serverObj *Server) SetEnableMining(enable bool) error {
	serverObj.nodeState.enableMining = enable
	return nil
}

func (serverObj *Server) GetEnableMining() bool {
	return serverObj.nodeState.enableMining
}

func (serverObj *Server) GetNodeMode() string {
	return serverObj.nodeState.nodeMode
}
