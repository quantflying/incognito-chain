package main

import "sync"

type nodeState struct {
	lock         sync.Mutex
	enableMining bool
	user         struct {
		miningKeys string
		privateKey string
	}
	nodeMode string
}
