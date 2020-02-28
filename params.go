package main

import "github.com/incognitochain/incognito-chain/blockchain_v2/params"

// activeNetParams is a pointer to the parameters specific to the
// currently active network.
var activeNetParams = &mainNetParams

// component is used to group parameters for various networks such as the main
// network and test networks.
type netparams struct {
	*params.Params
	rpcPort string
	wsPort  string
}

var mainNetParams = netparams{
	Params:  &params.ChainMainParam,
	rpcPort: MainnetRpcServerPort,
	wsPort:  MainnetWsServerPort,
}

var testNetParams = netparams{
	Params:  &params.ChainTestParam,
	rpcPort: TestnetRpcServerPort,
	wsPort:  TestnetWsServerPort,
}

// netName returns the name used when referring to a coin network.
func netName(chainParams *netparams) string {
	return chainParams.Name
}
