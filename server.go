package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/incognitochain/incognito-chain/blockchain_v2/params"
	"github.com/incognitochain/incognito-chain/peerv2"

	"github.com/incognitochain/incognito-chain/blockchain_v2"
	"github.com/incognitochain/incognito-chain/blockchain_v2/btc"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/memcache"
	"github.com/incognitochain/incognito-chain/pubsub"

	"github.com/incognitochain/incognito-chain/addrmanager"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/connmanager"

	//consensus "github.com/incognitochain/incognito-chain/consensus_v2"

	"github.com/incognitochain/incognito-chain/databasemp"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/netsync"
	"github.com/incognitochain/incognito-chain/peer"
	libp2p "github.com/libp2p/go-libp2p-peer"
)

// setupRPCListeners returns a slice of listeners that are configured for use
// with the RPC server depending on the configuration settings for listen
// addresses and TLS.
func (serverObj *Server) setupRPCListeners() ([]net.Listener, error) {
	// Setup TLS if not disabled.
	// listenFunc := net.Listen
	// if !cfg.DisableTLS {
	// 	Logger.log.Debug("Disable TLS for RPC is false")
	// 	// Generate the TLS cert and key file if both don't already
	// 	// exist.
	// 	if !fileExists(cfg.RPCKey) && !fileExists(cfg.RPCCert) {
	// 		err := rpcserver.GenCertPair(cfg.RPCCert, cfg.RPCKey)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// 	keyPair, err := tls.LoadX509KeyPair(cfg.RPCCert, cfg.RPCKey)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	tlsConfig := tls.Config{
	// 		Certificates: []tls.Certificate{keyPair},
	// 		MinVersion:   tls.VersionTLS12,
	// 	}

	// 	// Change the standard net.Listen function to the tls one.
	// 	listenFunc = func(net string, laddr string) (net.Listener, error) {
	// 		return tls.Listen(net, laddr, &tlsConfig)
	// 	}
	// } else {
	// 	Logger.log.Debug("Disable TLS for RPC is true")
	// }

	// netAddrs, err := common.ParseListeners(cfg.RPCListeners, "tcp")
	// if err != nil {
	// 	return nil, err
	// }

	// listeners := make([]net.Listener, 0, len(netAddrs))
	// for _, addr := range netAddrs {
	// 	listener, err := listenFunc(addr.Network(), addr.String())
	// 	if err != nil {
	// 		log.Printf("Can't listen on %s: %v", addr, err)
	// 		continue
	// 	}
	// 	listeners = append(listeners, listener)
	// }
	// return listeners, nil
	return nil, nil
}
func (serverObj *Server) setupRPCWsListeners() ([]net.Listener, error) {
	// Setup TLS if not disabled.
	// listenFunc := net.Listen
	// if !cfg.DisableTLS {
	// 	Logger.log.Debug("Disable TLS for RPC is false")
	// 	// Generate the TLS cert and key file if both don't already
	// 	// exist.
	// 	if !fileExists(cfg.RPCKey) && !fileExists(cfg.RPCCert) {
	// 		err := rpcserver.GenCertPair(cfg.RPCCert, cfg.RPCKey)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// 	keyPair, err := tls.LoadX509KeyPair(cfg.RPCCert, cfg.RPCKey)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	tlsConfig := tls.Config{
	// 		Certificates: []tls.Certificate{keyPair},
	// 		MinVersion:   tls.VersionTLS12,
	// 	}

	// 	// Change the standard net.Listen function to the tls one.
	// 	listenFunc = func(net string, laddr string) (net.Listener, error) {
	// 		return tls.Listen(net, laddr, &tlsConfig)
	// 	}
	// } else {
	// 	Logger.log.Debug("Disable TLS for RPC is true")
	// }

	// netAddrs, err := common.ParseListeners(cfg.RPCWSListeners, "tcp")
	// if err != nil {
	// 	return nil, err
	// }

	// listeners := make([]net.Listener, 0, len(netAddrs))
	// for _, addr := range netAddrs {
	// 	listener, err := listenFunc(addr.Network(), addr.String())
	// 	if err != nil {
	// 		log.Printf("Can't listen on %s: %v", addr, err)
	// 		continue
	// 	}
	// 	listeners = append(listeners, listener)
	// }
	// return listeners, nil
	return nil, nil
}

/*
NewServer - create server object which control all process of node
*/
func (serverObj *Server) NewServer(listenAddrs string, db incdb.Database, dbmp databasemp.DatabaseInterface, chainParams *params.Params, protocolVer string, interrupt <-chan struct{}) error {
	// Init data for Server
	serverObj.protocolVersion = protocolVer
	serverObj.chainParams = chainParams
	serverObj.cQuit = make(chan struct{})
	serverObj.cNewPeers = make(chan *peer.Peer)
	serverObj.dataBase = db
	serverObj.memCache = memcache.New()
	serverObj.consensusEngine = consensus.New()

	//Init channel
	cPendingTxs := make(chan metadata.Transaction, 500)
	cRemovedTxs := make(chan metadata.Transaction, 500)

	var err error
	// init an pubsub manager
	var pubsubManager = pubsub.NewPubSubManager()

	serverObj.miningKeys = cfg.MiningKeys
	serverObj.privateKey = cfg.PrivateKey
	if serverObj.miningKeys == "" && serverObj.privateKey == "" {
		if cfg.NodeMode == common.NodeModeAuto || cfg.NodeMode == common.NodeModeBeacon || cfg.NodeMode == common.NodeModeShard {
			panic("miningkeys can't be empty in this node mode")
		}
	}
	serverObj.pubsubManager = pubsubManager
	// serverObj.beaconPool = mempool.GetBeaconPool()
	// serverObj.shardToBeaconPool = mempool.GetShardToBeaconPool()
	// serverObj.crossShardPool = make(map[byte]blockchain_v2.CrossShardPool)
	// serverObj.shardPool = make(map[byte]blockchain_v2.ShardPool)
	serverObj.blockChain = &blockchain_v2.Blockchain{}
	serverObj.isEnableMining = cfg.EnableMining
	// create mempool tx
	serverObj.memPool = &mempool.TxPool{}

	relayShards := []byte{}
	if cfg.RelayShards == "all" {
		for index := 0; index < common.MaxShardNumber; index++ {
			relayShards = append(relayShards, byte(index))
		}
	} else {
		var validPath = regexp.MustCompile(`(?s)[[:digit:]]+`)
		relayShardsStr := validPath.FindAllString(cfg.RelayShards, -1)
		for index := 0; index < len(relayShardsStr); index++ {
			s, err := strconv.Atoi(relayShardsStr[index])
			if err == nil {
				relayShards = append(relayShards, byte(s))
			}
		}
	}
	var randomClient btc.RandomClient
	if cfg.BtcClient == 0 {
		randomClient = &btc.BlockCypherClient{}
		Logger.log.Info("Init 3-rd Party Random Client")

	} else {
		if cfg.BtcClientIP == common.EmptyString || cfg.BtcClientUsername == common.EmptyString || cfg.BtcClientPassword == common.EmptyString {
			Logger.log.Error("Please input Bitcoin Client Ip, Username, password. Otherwise, set btcclient is 0 or leave it to default value")
			os.Exit(2)
		}
		randomClient = btc.NewBTCClient(cfg.BtcClientUsername, cfg.BtcClientPassword, cfg.BtcClientIP, cfg.BtcClientPort)
		Logger.log.Infof("Init Bitcoin Core Client with IP %+v, Port %+v, Username %+v, Password %+v", cfg.BtcClientIP, cfg.BtcClientPort, cfg.BtcClientUsername, cfg.BtcClientPassword)
	}
	// Init block template generator
	serverObj.blockgen, err = blockchain_v2.NewBlockGenerator(serverObj.memPool, serverObj.blockChain, serverObj.shardToBeaconPool, serverObj.crossShardPool, cPendingTxs, cRemovedTxs)
	if err != nil {
		return err
	}
	// Init consensus engine
	err = serverObj.consensusEngine.Init(&consensus.EngineConfig{
		Blockchain:    serverObj.blockChain,
		Node:          serverObj,
		PubSubManager: serverObj.pusubManager,
	})
	if err != nil {
		return err
	}
	// TODO hy
	// Connect to highway
	Logger.log.Debug("Listenner: ", cfg.Listener)
	Logger.log.Debug("Bootnode: ", cfg.DiscoverPeersAddress)
	Logger.log.Debug("PrivateKey: ", cfg.PrivateKey)

	ip, port := peerv2.ParseListenner(cfg.Listener, "127.0.0.1", 9433)
	host := peerv2.NewHost(version(), ip, port, cfg.Libp2pPrivateKey)

	miningKeys := serverObj.consensusEngine.GetMiningPublicKeys()
	pubkey := miningKeys[common.BlsConsensus]
	dispatcher := &peerv2.Dispatcher{
		MessageListeners: &peerv2.MessageListeners{
			OnBlockShard:       serverObj.OnBlockShard,
			OnBlockBeacon:      serverObj.OnBlockBeacon,
			OnCrossShard:       serverObj.OnCrossShard,
			OnShardToBeacon:    serverObj.OnShardToBeacon,
			OnTx:               serverObj.OnTx,
			OnTxPrivacyToken:   serverObj.OnTxPrivacyToken,
			OnVersion:          serverObj.OnVersion,
			OnGetBlockBeacon:   serverObj.OnGetBlockBeacon,
			OnGetBlockShard:    serverObj.OnGetBlockShard,
			OnGetCrossShard:    serverObj.OnGetCrossShard,
			OnGetShardToBeacon: serverObj.OnGetShardToBeacon,
			OnVerAck:           serverObj.OnVerAck,
			OnGetAddr:          serverObj.OnGetAddr,
			OnAddr:             serverObj.OnAddr,

			//mubft
			OnBFTMsg:    serverObj.OnBFTMsg,
			OnPeerState: serverObj.OnPeerState,
		},
	}

	// metrics.SetGlobalParam("Bootnode", cfg.DiscoverPeersAddress)
	// metrics.SetGlobalParam("ExternalAddress", cfg.ExternalAddress)

	serverObj.highway = peerv2.NewConnManager(
		host,
		cfg.DiscoverPeersAddress,
		&pubkey,
		serverObj.consensusEngine,
		dispatcher,
		cfg.NodeMode,
		relayShards,
	)

	err = serverObj.blockChain.Init(&blockchain_v2.Config{
		ChainParams: serverObj.chainParams,
		DataBase:    serverObj.dataBase,
		MemCache:    serverObj.memCache,
		//MemCache:          nil,
		BlockGen:          serverObj.blockgen,
		Interrupt:         interrupt,
		RelayShards:       relayShards,
		BeaconPool:        serverObj.beaconPool,
		ShardPool:         serverObj.shardPool,
		ShardToBeaconPool: serverObj.shardToBeaconPool,
		CrossShardPool:    serverObj.crossShardPool,
		Server:            serverObj,
		// UserKeySet:        serverObj.userKeySet,
		NodeMode:        cfg.NodeMode,
		FeeEstimator:    make(map[byte]blockchain_v2.FeeEstimator),
		PubSubManager:   pubsubManager,
		RandomClient:    randomClient,
		ConsensusEngine: serverObj.consensusEngine,
		Highway:         serverObj.highway,
	})
	if err != nil {
		return err
	}
	serverObj.blockChain.InitChannelBlockchain(cRemovedTxs)
	if err != nil {
		return err
	}
	//init beacon pol
	mempool.InitBeaconPool(serverObj.pusubManager)
	//init shard pool
	mempool.InitShardPool(serverObj.shardPool, serverObj.pusubManager)
	//init cross shard pool
	mempool.InitCrossShardPool(serverObj.crossShardPool, db, serverObj.blockChain)

	//init shard to beacon bool
	mempool.InitShardToBeaconPool()
	// or if it cannot be loaded, create a new one.
	if cfg.FastStartup {
		// Logger.log.Debug("Load chain dependencies from DB")
		// serverObj.feeEstimator = make(map[byte]*mempool.FeeEstimator)
		// for shardID, bestState := range serverObj.blockChain.BestState.Shard {
		// 	_ = bestState
		// 	feeEstimatorData, err := rawdbv2.GetFeeEstimator(serverObj.dataBase, shardID)
		// 	if err == nil && len(feeEstimatorData) > 0 {
		// 		feeEstimator, err := mempool.RestoreFeeEstimator(feeEstimatorData)
		// 		if err != nil {
		// 			Logger.log.Debugf("Failed to restore fee estimator %v", err)
		// 			Logger.log.Debug("Init NewFeeEstimator")
		// 			serverObj.feeEstimator[shardID] = mempool.NewFeeEstimator(
		// 				mempool.DefaultEstimateFeeMaxRollback,
		// 				mempool.DefaultEstimateFeeMinRegisteredBlocks,
		// 				cfg.LimitFee)
		// 		} else {
		// 			serverObj.feeEstimator[shardID] = feeEstimator
		// 		}
		// 	} else {
		// 		Logger.log.Debugf("Failed to get fee estimator from DB %v", err)
		// 		Logger.log.Debug("Init NewFeeEstimator")
		// 		serverObj.feeEstimator[shardID] = mempool.NewFeeEstimator(
		// 			mempool.DefaultEstimateFeeMaxRollback,
		// 			mempool.DefaultEstimateFeeMinRegisteredBlocks,
		// 			cfg.LimitFee)
		// 	}
		// }
	} else {
		//err := rawdb.CleanCommitments(serverObj.dataBase)
		//if err != nil {
		//	Logger.log.Error(err)
		//	return err
		//}
		//err = rawdb.CleanSerialNumbers(serverObj.dataBase)
		//if err != nil {
		//	Logger.log.Error(err)
		//	return err
		//}
		//err = rawdb.CleanFeeEstimator(serverObj.dataBase)
		//if err != nil {
		//	Logger.log.Error(err)
		//	return err
		//}
		// serverObj.feeEstimator = make(map[byte]*mempool.FeeEstimator)
	}
	for shardID, feeEstimator := range serverObj.feeEstimator {
		serverObj.blockChain.SetFeeEstimator(feeEstimator, shardID)
	}

	serverObj.memPool.Init(&mempool.Config{
		BlockChain:        serverObj.blockChain,
		DataBase:          serverObj.dataBase,
		ChainParams:       chainParams,
		FeeEstimator:      serverObj.feeEstimator,
		TxLifeTime:        cfg.TxPoolTTL,
		MaxTx:             cfg.TxPoolMaxTx,
		DataBaseMempool:   dbmp,
		IsLoadFromMempool: cfg.LoadMempool,
		PersistMempool:    cfg.PersistMempool,
		RelayShards:       relayShards,
		// UserKeyset:        serverObj.userKeySet,
		PubSubManager: serverObj.pusubManager,
	})
	serverObj.memPool.AnnouncePersisDatabaseMempool()
	//add tx pool
	serverObj.blockChain.AddTxPool(serverObj.memPool)
	serverObj.memPool.InitChannelMempool(cPendingTxs, cRemovedTxs)
	//==============Temp mem pool only used for validation
	serverObj.tempMemPool = &mempool.TxPool{}
	serverObj.tempMemPool.Init(&mempool.Config{
		BlockChain:    serverObj.blockChain,
		DataBase:      serverObj.dataBase,
		ChainParams:   chainParams,
		FeeEstimator:  serverObj.feeEstimator,
		MaxTx:         cfg.TxPoolMaxTx,
		PubSubManager: pubsubManager,
	})
	go serverObj.tempMemPool.Start(serverObj.cQuit)
	serverObj.blockChain.AddTempTxPool(serverObj.tempMemPool)
	//===============

	serverObj.addrManager = addrmanager.NewAddrManager(cfg.DataDir, common.HashH(common.Uint32ToBytes(activeNetParams.Params.Net))) // use network param Net as key for storage

	// Init Net Sync manager to process messages
	serverObj.netSync = &netsync.NetSync{}
	serverObj.netSync.Init(&netsync.NetSyncConfig{
		BlockChain:        serverObj.blockChain,
		ChainParam:        chainParams,
		TxMemPool:         serverObj.memPool,
		Server:            serverObj,
		Consensus:         serverObj.consensusEngine, // for onBFTMsg
		ShardToBeaconPool: serverObj.shardToBeaconPool,
		CrossShardPool:    serverObj.crossShardPool,
		PubSubManager:     serverObj.pusubManager,
		RelayShard:        relayShards,
		RoleInCommittees:  -1,
	})
	// Create a connection manager.
	var listenPeer *peer.Peer
	if !cfg.DisableListen {
		var err error

		// this is initializing our listening peer
		listenPeer, err = serverObj.InitListenerPeer(serverObj.addrManager, listenAddrs)
		if err != nil {
			Logger.log.Error(err)
			return err
		}
	}
	isRelayNodeForConsensus := cfg.Accelerator
	if isRelayNodeForConsensus {
		cfg.MaxPeersSameShard = 9999
		cfg.MaxPeersOtherShard = 9999
		cfg.MaxPeersOther = 9999
		cfg.MaxPeersNoShard = 0
		cfg.MaxPeersBeacon = 9999
	}
	connManager := connmanager.New(&connmanager.Config{
		OnInboundAccept:      serverObj.InboundPeerConnected,
		OnOutboundConnection: serverObj.OutboundPeerConnected,
		ListenerPeer:         listenPeer,
		DiscoverPeers:        cfg.DiscoverPeers,
		DiscoverPeersAddress: cfg.DiscoverPeersAddress,
		ExternalAddress:      cfg.ExternalAddress,
		// config for connection of shard
		MaxPeersSameShard:  cfg.MaxPeersSameShard,
		MaxPeersOtherShard: cfg.MaxPeersOtherShard,
		MaxPeersOther:      cfg.MaxPeersOther,
		MaxPeersNoShard:    cfg.MaxPeersNoShard,
		MaxPeersBeacon:     cfg.MaxPeersBeacon,
	})
	serverObj.connManager = connManager

	// Start up persistent peers.
	permanentPeers := cfg.ConnectPeers
	if len(permanentPeers) == 0 {
		permanentPeers = cfg.AddPeers
	}

	for _, addr := range permanentPeers {
		go serverObj.connManager.Connect(addr, "", "", nil)
	}

	if !cfg.DisableRPC {
		// Setup listeners for the configured RPC listen addresses and
		// TLS settings.
		fmt.Println("settingup RPCListeners")
		httpListeners, err := serverObj.setupRPCListeners()
		wsListeners, err := serverObj.setupRPCWsListeners()
		if err != nil {
			return err
		}
		if len(httpListeners) == 0 && len(wsListeners) == 0 {
			return errors.New("RPCS: No valid listen address")
		}

		// rpcConfig := rpcserver.RpcServerConfig{
		// 	HttpListenters:  httpListeners,
		// 	WsListenters:    wsListeners,
		// 	RPCQuirks:       cfg.RPCQuirks,
		// 	RPCMaxClients:   cfg.RPCMaxClients,
		// 	RPCMaxWSClients: cfg.RPCMaxWSClients,
		// 	ChainParams:     chainParams,
		// 	BlockChain:      serverObj.blockChain,
		// 	Blockgen:        serverObj.blockgen,
		// 	TxMemPool:       serverObj.memPool,
		// 	Server:          serverObj,
		// 	Wallet:          serverObj.wallet,
		// 	ConnMgr:         serverObj.connManager,
		// 	AddrMgr:         serverObj.addrManager,
		// 	RPCUser:         cfg.RPCUser,
		// 	RPCPass:         cfg.RPCPass,
		// 	RPCLimitUser:    cfg.RPCLimitUser,
		// 	RPCLimitPass:    cfg.RPCLimitPass,
		// 	DisableAuth:     cfg.RPCDisableAuth,
		// 	NodeMode:        cfg.NodeMode,
		// 	FeeEstimator:    serverObj.feeEstimator,
		// 	ProtocolVersion: serverObj.protocolVersion,
		// 	Database:        &serverObj.dataBase,
		// 	MiningKeys:      cfg.MiningKeys,
		// 	NetSync:         serverObj.netSync,
		// 	PubSubManager:   pubsubManager,
		// 	ConsensusEngine: serverObj.consensusEngine,
		// 	MemCache:        serverObj.memCache,
		// }
		// serverObj.rpcServer = &rpcserver.RpcServer{}
		// serverObj.rpcServer.Init(&rpcConfig)

		// // init rpc client instance and stick to Blockchain object
		// // in order to communicate to external services (ex. eth light node)
		// //serverObj.blockChain.SetRPCClientChain(rpccaller.NewRPCClient())

		// // Signal process shutdown when the RPC server requests it.
		// go func() {
		// 	<-serverObj.rpcServer.RequestedProcessShutdown()
		// 	shutdownRequestChannel <- struct{}{}
		// }()
	}

	//Init Metric Tool
	//if cfg.MetricUrl != "" {
	//	grafana := metrics.NewGrafana(cfg.MetricUrl, cfg.ExternalAddress)
	//	metrics.InitMetricTool(&grafana)
	//}
	return nil
}

/*
// InboundPeerConnected is invoked by the connection manager when a new
// inbound connection is established.
*/
func (serverObj *Server) InboundPeerConnected(peerConn *peer.PeerConn) {
	Logger.log.Debug("inbound connected")
}

/*
// outboundPeerConnected is invoked by the connection manager when a new
// outbound connection is established.  It initializes a new outbound server
// peer instance, associates it with the relevant state such as the connection
// request instance and the connection itserverObj, and finally notifies the address
// manager of the attempt.
*/
func (serverObj *Server) OutboundPeerConnected(peerConn *peer.PeerConn) {
	Logger.log.Debug("Outbound PEER connected with PEER Id - " + peerConn.GetRemotePeerID().Pretty())
	err := serverObj.PushVersionMessage(peerConn)
	if err != nil {
		Logger.log.Error(err)
	}
}

/*
// peerHandler is used to handle peer operations such as adding and removing
// peers to and from the server, banning peers, and broadcasting messages to
// peers.  It must be run in a goroutine.
*/
func (serverObj *Server) peerHandler() {
	// Start the address manager and sync manager, both of which are needed
	// by peers.  This is done here since their lifecycle is closely tied
	// to this handler and rather than adding more channels to sychronize
	// things, it's easier and slightly faster to simply start and stop them
	// in this handler.
	serverObj.addrManager.Start()
	serverObj.netSync.Start()

	Logger.log.Debug("Start peer handler")

	if len(cfg.ConnectPeers) == 0 {
		for _, addr := range serverObj.addrManager.AddressCache() {
			pk, pkT := addr.GetPublicKey()
			go serverObj.connManager.Connect(addr.GetRawAddress(), pk, pkT, nil)
		}
	}

	go serverObj.connManager.Start(cfg.DiscoverPeersAddress)

out:
	for {
		select {
		case p := <-serverObj.cNewPeers:
			serverObj.handleAddPeerMsg(p)
		case <-serverObj.cQuit:
			{
				break out
			}
		}
	}
	serverObj.netSync.Stop()
	errStopAddrManager := serverObj.addrManager.Stop()
	if errStopAddrManager != nil {
		Logger.log.Error(errStopAddrManager)
	}
	errStopConnManager := serverObj.connManager.Stop()
	if errStopAddrManager != nil {
		Logger.log.Error(errStopConnManager)
	}
}

func (serverObj *Server) GetActiveShardNumber() int {
	return serverObj.blockChain.BestState.Beacon.ActiveShards
}

/*
// initListeners initializes the configured net listeners and adds any bound
// addresses to the address manager. Returns the listeners and a NAT interface,
// which is non-nil if UPnP is in use.
*/
func (serverObj *Server) InitListenerPeer(amgr *addrmanager.AddrManager, listenAddrs string) (*peer.Peer, error) {
	netAddr, err := common.ParseListener(listenAddrs, "ip")
	if err != nil {
		return nil, err
	}

	// use keycache to save listener peer into file, this will make peer id of listener not change after turn off node
	kc := KeyCache{}
	kc.Load(filepath.Join(cfg.DataDir, "listenerpeer.json"))

	// load seed of libp2p from keycache file, if not exist -> save a new data into keycache file
	seed := int64(0)
	seedC, _ := strconv.ParseInt(os.Getenv("LISTENER_PEER_SEED"), 10, 64)
	if seedC == 0 {
		key := "LISTENER_PEER_SEED"
		seedT := kc.Get(key)
		if seedT == nil {
			seed = common.RandInt64()
			kc.Set(key, seed)
		} else {
			seed = int64(seedT.(float64))
		}
	} else {
		seed = seedC
	}

	peerObj := peer.Peer{}
	peerObj.SetSeed(seed)
	peerObj.SetListeningAddress(*netAddr)
	peerObj.SetPeerConns(nil)
	peerObj.SetPendingPeers(nil)
	peerObj.SetConfig(*serverObj.NewPeerConfig())
	err = peerObj.Init(peer.PrefixProtocolID + version()) // it should be /incognito/x.yy.zz-beta
	if err != nil {
		return nil, err
	}

	kc.Save()
	return &peerObj, nil
}

/*
// newPeerConfig returns the configuration for the listening RemotePeer.
*/
func (serverObj *Server) NewPeerConfig() *peer.Config {
	// KeySetUser := serverObj.userKeySet
	config := &peer.Config{
		MessageListeners: peer.MessageListeners{
			OnBlockShard:       serverObj.OnBlockShard,
			OnBlockBeacon:      serverObj.OnBlockBeacon,
			OnCrossShard:       serverObj.OnCrossShard,
			OnShardToBeacon:    serverObj.OnShardToBeacon,
			OnTx:               serverObj.OnTx,
			OnTxPrivacyToken:   serverObj.OnTxPrivacyToken,
			OnVersion:          serverObj.OnVersion,
			OnGetBlockBeacon:   serverObj.OnGetBlockBeacon,
			OnGetBlockShard:    serverObj.OnGetBlockShard,
			OnGetCrossShard:    serverObj.OnGetCrossShard,
			OnGetShardToBeacon: serverObj.OnGetShardToBeacon,
			OnVerAck:           serverObj.OnVerAck,
			OnGetAddr:          serverObj.OnGetAddr,
			OnAddr:             serverObj.OnAddr,

			//mubft
			OnBFTMsg: serverObj.OnBFTMsg,
			// OnInvalidBlock:  serverObj.OnInvalidBlock,
			OnPeerState: serverObj.OnPeerState,
			//
			PushRawBytesToShard:  serverObj.PushRawBytesToShard,
			PushRawBytesToBeacon: serverObj.PushRawBytesToBeacon,
			GetCurrentRoleShard:  serverObj.GetCurrentRoleShard,
		},
		MaxInPeers:      cfg.MaxInPeers,
		MaxPeers:        cfg.MaxPeers,
		MaxOutPeers:     cfg.MaxOutPeers,
		ConsensusEngine: serverObj.consensusEngine,
	}
	// if KeySetUser != nil && len(KeySetUser.PrivateKey) != 0 {
	// 	config.UserKeySet = KeySetUser
	// }
	return config
}

func (serverObj *Server) GetPeerIDsFromPublicKey(pubKey string) []libp2p.ID {
	result := []libp2p.ID{}
	// panic(pubKey)
	// os.Exit(0)
	listener := serverObj.connManager.GetConfig().ListenerPeer
	for _, peerConn := range listener.GetPeerConns() {
		// Logger.log.Debug("Test PeerConn", peerConn.RemotePeer.PaymentAddress)
		pk, _ := peerConn.GetRemotePeer().GetPublicKey()
		if pk == pubKey {
			exist := false
			for _, item := range result {
				if item.Pretty() == peerConn.GetRemotePeer().GetPeerID().Pretty() {
					exist = true
				}
			}

			if !exist {
				result = append(result, peerConn.GetRemotePeer().GetPeerID())
			}
		}
	}

	return result
}

func (serverObj *Server) GetNodeRole() string {
	if serverObj.miningKeys == "" && serverObj.privateKey == "" {
		return ""
	}
	if cfg.NodeMode == "relay" {
		return "RELAY"
	}
	role, shardID := serverObj.consensusEngine.GetUserLayer()

	switch shardID {
	case -2:
		return ""
	case -1:
		return "BEACON_" + role
	default:
		return "SHARD_" + role
	}
	// pubkey := serverObj.userKeySet.GetPublicKeyInBase58CheckEncode()
	// if common.IndexOfStr(pubkey, blockchain.GetBestStateBeacon().BeaconCommittee) > -1 {
	// 	return "BEACON_VALIDATOR"
	// }
	// if common.IndexOfStr(pubkey, blockchain.GetBestStateBeacon().BeaconPendingValidator) > -1 {
	// 	return "BEACON_WAITING"
	// }
	// shardCommittee := blockchain.GetBestStateBeacon().GetShardCommittee()
	// for _, s := range shardCommittee {
	// 	if common.IndexOfStr(pubkey, s) > -1 {
	// 		return "SHARD_VALIDATOR"
	// 	}
	// }
	// shardPendingCommittee := blockchain.GetBestStateBeacon().GetShardPendingValidator()
	// for _, s := range shardPendingCommittee {
	// 	if common.IndexOfStr(pubkey, s) > -1 {
	// 		return "SHARD_VALIDATOR"
	// 	}
	// }
}

func (serverObj *Server) GetCurrentRoleShard() (string, *byte) {
	return serverObj.connManager.GetCurrentRoleShard()
}

func (serverObj *Server) UpdateConsensusState(role string, userPbk string, currentShard *byte, beaconCommittee []string, shardCommittee map[byte][]string) {
	changed := serverObj.connManager.UpdateConsensusState(role, userPbk, currentShard, beaconCommittee, shardCommittee)
	if changed {
		Logger.log.Debug("UpdateConsensusState is true")
	} else {
		Logger.log.Debug("UpdateConsensusState is false")
	}
}

func (serverObj *Server) GetChainMiningStatus(chain int) string {
	const (
		notmining = "notmining"
		syncing   = "syncing"
		ready     = "ready"
		mining    = "mining"
		pending   = "pending"
		waiting   = "waiting"
	)
	if chain >= common.MaxShardNumber || chain < -1 {
		return notmining
	}
	if cfg.MiningKeys != "" || cfg.PrivateKey != "" {
		//Beacon: chain = -1
		layer, role, shardID := serverObj.consensusEngine.GetUserRole()

		if shardID == -2 {
			return notmining
		}
		if chain != -1 && layer == common.BeaconRole {
			return notmining
		}

		switch layer {
		case common.BeaconRole:
			switch role {
			case common.CommitteeRole:
				if serverObj.blockChain.Synker.IsLatest(false, 0) {
					return mining
				}
				return syncing
			case common.PendingRole:
				return pending
			case common.WaitingRole:
				return waiting
			}
		case common.ShardRole:
			if chain != shardID {
				return notmining
			}
			switch role {
			case common.CommitteeRole:
				if serverObj.blockChain.Synker.IsLatest(true, byte(chain)) {
					return mining
				}
				return syncing
			case common.PendingRole:
				return pending
			case common.WaitingRole:
				return waiting
			}
		default:
			return notmining
		}

	}
	return notmining
}

func (serverObj *Server) GetMiningKeys() string {
	return serverObj.miningKeys
}

func (serverObj *Server) GetPrivateKey() string {
	return serverObj.privateKey
}

func (serverObj *Server) DropAllConnections() {
	serverObj.connManager.DropAllConnections()
}

func (serverObj *Server) GetPublicKeyRole(publicKey string, keyType string) (int, int) {
	// var beaconBestState blockchain.BeaconBestState
	// err := beaconBestState.CloneBeaconBestStateFrom(serverObj.blockChain.BestState.Beacon)
	// if err != nil {
	// 	return -2, -1
	// }
	// for shardID, pubkeyArr := range beaconBestState.ShardPendingValidator {
	// 	keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(pubkeyArr, keyType)
	// 	found := common.IndexOfStr(publicKey, keyList)
	// 	if found > -1 {
	// 		return 0, int(shardID)
	// 	}
	// }
	// for shardID, pubkeyArr := range beaconBestState.ShardCommittee {
	// 	keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(pubkeyArr, keyType)
	// 	found := common.IndexOfStr(publicKey, keyList)
	// 	if found > -1 {
	// 		return 1, int(shardID)
	// 	}
	// }

	// keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.BeaconCommittee, keyType)
	// found := common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 1, -1
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.BeaconPendingValidator, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 0, -1
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateBeaconWaitingForCurrentRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 0, -1
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateBeaconWaitingForNextRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 0, -1
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateShardWaitingForCurrentRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 0, -1
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateShardWaitingForNextRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return 0, -1
	// }

	return -1, -1
}

func (serverObj *Server) GetIncognitoPublicKeyRole(publicKey string) (int, bool, int) {
	// var beaconBestState blockchain.BeaconBestState
	// err := beaconBestState.CloneBeaconBestStateFrom(serverObj.blockChain.BestState.Beacon)
	// if err != nil {
	// 	return -2, false, -1
	// }

	// for shardID, pubkeyArr := range beaconBestState.ShardPendingValidator {
	// 	for _, key := range pubkeyArr {
	// 		if key.GetIncKeyBase58() == publicKey {
	// 			return 1, false, int(shardID)
	// 		}
	// 	}
	// }
	// for shardID, pubkeyArr := range beaconBestState.ShardCommittee {
	// 	for _, key := range pubkeyArr {
	// 		if key.GetIncKeyBase58() == publicKey {
	// 			return 2, false, int(shardID)
	// 		}
	// 	}
	// }

	// for _, key := range beaconBestState.BeaconCommittee {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 2, true, -1
	// 	}
	// }

	// for _, key := range beaconBestState.BeaconPendingValidator {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 1, true, -1
	// 	}
	// }

	// for _, key := range beaconBestState.CandidateBeaconWaitingForCurrentRandom {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 0, true, -1
	// 	}
	// }

	// for _, key := range beaconBestState.CandidateBeaconWaitingForNextRandom {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 0, true, -1
	// 	}
	// }

	// for _, key := range beaconBestState.CandidateShardWaitingForCurrentRandom {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 0, false, -1
	// 	}
	// }
	// for _, key := range beaconBestState.CandidateShardWaitingForNextRandom {
	// 	if key.GetIncKeyBase58() == publicKey {
	// 		return 0, false, -1
	// 	}
	// }

	return -1, false, -1
}

func (serverObj *Server) GetMinerIncognitoPublickey(publicKey string, keyType string) []byte {
	// var beaconBestState blockchain.BeaconBestState
	// err := beaconBestState.CloneBeaconBestStateFrom(serverObj.blockChain.BestState.Beacon)
	// if err != nil {
	// 	return nil
	// }
	// for _, pubkeyArr := range beaconBestState.ShardPendingValidator {
	// 	keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(pubkeyArr, keyType)
	// 	found := common.IndexOfStr(publicKey, keyList)
	// 	if found > -1 {
	// 		return pubkeyArr[found].GetNormalKey()
	// 	}
	// }
	// for _, pubkeyArr := range beaconBestState.ShardCommittee {
	// 	keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(pubkeyArr, keyType)
	// 	found := common.IndexOfStr(publicKey, keyList)
	// 	if found > -1 {
	// 		return pubkeyArr[found].GetNormalKey()
	// 	}
	// }

	// keyList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.BeaconCommittee, keyType)
	// found := common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.BeaconCommittee[found].GetNormalKey()
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.BeaconPendingValidator, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.BeaconPendingValidator[found].GetNormalKey()
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateBeaconWaitingForCurrentRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.CandidateBeaconWaitingForCurrentRandom[found].GetNormalKey()
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateBeaconWaitingForNextRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.CandidateBeaconWaitingForNextRandom[found].GetNormalKey()
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateShardWaitingForCurrentRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.CandidateShardWaitingForCurrentRandom[found].GetNormalKey()
	// }

	// keyList, _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(beaconBestState.CandidateShardWaitingForNextRandom, keyType)
	// found = common.IndexOfStr(publicKey, keyList)
	// if found > -1 {
	// 	return beaconBestState.CandidateShardWaitingForNextRandom[found].GetNormalKey()
	// }

	return nil
}

func (serverObj *Server) GetIncognitoPublicKeyStatus(publicKey string) (int, bool, int) {
	panic("implement me")
}
