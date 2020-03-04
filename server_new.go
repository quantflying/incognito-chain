package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/incognitochain/incognito-chain/addrmanager"
	"github.com/incognitochain/incognito-chain/blockchain_v2"
	"github.com/incognitochain/incognito-chain/blockchain_v2/params"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/connmanager"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/netsync"
	"github.com/incognitochain/incognito-chain/peer"
	"github.com/incognitochain/incognito-chain/peerv2"
	"github.com/incognitochain/incognito-chain/pubsub"
	"github.com/incognitochain/incognito-chain/wallet"
)

type Server struct {
	started     int32
	startupTime int64

	protocolVersion string
	chainParams     *params.Params
	connManager     *connmanager.ConnManager
	blockChain      *blockchain_v2.Blockchain
	dataBase        incdb.Database
	memCache        *memcache.MemoryCache
	// rpcServer         *rpcserver.RpcServer
	// memPool           *mempool.TxPool
	// tempMemPool       *mempool.TxPool
	// beaconPool        *mempool.BeaconPool
	// shardPool         map[byte]blockchain_v2.ShardPool
	// shardToBeaconPool *mempool.ShardToBeaconPool
	// crossShardPool    map[byte]blockchain_v2.CrossShardPool
	waitGroup       sync.WaitGroup
	netSync         *netsync.NetSync
	addrManager     *addrmanager.AddrManager
	wallet          *wallet.Wallet
	consensusEngine *consensus.Engine
	pubsubManager   *pubsub.PubSubManager
	// The fee estimator keeps track of how long transactions are left in
	// the mempool before they are mined into blocks.
	feeEstimator map[byte]*mempool.FeeEstimator
	highway      *peerv2.ConnManager

	cQuit     chan struct{}
	cNewPeers chan *peer.Peer

	nodeState *nodeState

	//TO BE MOVE
	isEnableMining bool
	miningKeys     string
	privateKey     string
}

/*
// WaitForShutdown blocks until the main listener and peer handlers are stopped.
*/
func (serverObj *Server) WaitForShutdown() {
	serverObj.waitGroup.Wait()
}

/*
// Stop gracefully shuts down the connection manager.
*/
func (serverObj *Server) Stop() error {
	// stop connManager
	errStopConnManager := serverObj.connManager.Stop()
	if errStopConnManager != nil {
		Logger.log.Error(errStopConnManager)
	}

	// Shutdown the RPC server if it's not disabled.
	// if !cfg.DisableRPC && serverObj.rpcServer != nil {
	// 	serverObj.rpcServer.Stop()
	// }

	// Save fee estimator in the db
	for shardID, feeEstimator := range serverObj.feeEstimator {
		Logger.log.Infof("Fee estimator data when saving #%d", feeEstimator)
		feeEstimatorData := feeEstimator.Save()
		if len(feeEstimatorData) > 0 {
			err := rawdbv2.StoreFeeEstimator(serverObj.dataBase, feeEstimatorData, shardID)
			if err != nil {
				Logger.log.Errorf("Can't save fee estimator data on chain #%d: %v", shardID, err)
			} else {
				Logger.log.Infof("Save fee estimator data on chain #%d", shardID)
			}
		}
	}

	err := serverObj.consensusEngine.Stop("all")
	if err != nil {
		Logger.log.Error(err)
	}
	// Signal the remaining goroutines to cQuit.
	close(serverObj.cQuit)
	return nil
}

/*
// Start begins accepting connections from peers.
*/
func (serverObj Server) Start() {
	// Already started?
	if atomic.AddInt32(&serverObj.started, 1) != 1 {
		return
	}
	Logger.log.Debug("Starting server")
	// --- Checkforce update code ---
	if serverObj.chainParams.CheckForce {
		serverObj.CheckForceUpdateSourceCode()
	}
	if cfg.IsTestnet() {
		Logger.log.Critical("************************" +
			"* Testnet is active *" +
			"************************")
	}
	// Server startup time. Used for the uptime command for uptime calculation.
	serverObj.startupTime = time.Now().Unix()

	// Start the peer handler which in turn starts the address and block
	// managers.
	serverObj.waitGroup.Add(1)

	serverObj.netSync.Start()

	go serverObj.highway.Start(serverObj.netSync)

	// if !cfg.DisableRPC && serverObj.rpcServer != nil {
	// 	serverObj.waitGroup.Add(1)

	// ----- REPLACED BY TransactionPoolBroadcastLoop -----
	// 	// Start the rebroadcastHandler, which ensures user tx received by
	// 	// the RPC server are rebroadcast until being included in a block.
	// 	//go serverObj.rebroadcastHandler()
	// ----- REPLACED BY TransactionPoolBroadcastLoop -----

	// 	serverObj.rpcServer.Start()
	// }

	err := serverObj.consensusEngine.Start()
	if err != nil {
		Logger.log.Error(err)
		go serverObj.Stop()
		return
	}
	if cfg.NodeMode != common.NodeModeRelay {
		serverObj.memPool.IsBlockGenStarted = true
		serverObj.blockChain.SetIsBlockGenStarted(true)
		for _, shardPool := range serverObj.shardPool {
			go shardPool.Start(serverObj.cQuit)
		}
		go serverObj.beaconPool.Start(serverObj.cQuit)
	}

	go serverObj.blockChain.Synker.Start()
	if serverObj.memPool != nil {
		err := serverObj.memPool.LoadOrResetDatabaseMempool()
		if err != nil {
			Logger.log.Error(err)
		}
		go serverObj.TransactionPoolBroadcastLoop()
		go serverObj.memPool.Start(serverObj.cQuit)
		go serverObj.memPool.MonitorPool()
	}
	go serverObj.pubsubManager.Start()
	// go metrics.StartSystemMetrics()
}
