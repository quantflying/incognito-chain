package main

import (
	"errors"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metrics"
	"github.com/incognitochain/incognito-chain/peer"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wire"
	libp2p "github.com/libp2p/go-libp2p-peer"
)

// OnBlock is invoked when a peer receives a block message.  It
// blocks until the coin block has been fully processed.
func (serverObj *Server) OnBlockShard(p *peer.PeerConn,
	msg *wire.MessageBlockShard) {
	Logger.log.Debug("Receive a new blockshard START")

	var txProcessed chan struct{}
	serverObj.netSync.QueueBlock(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new blockshard END")
}

func (serverObj *Server) OnBlockBeacon(p *peer.PeerConn,
	msg *wire.MessageBlockBeacon) {
	Logger.log.Debug("Receive a new blockbeacon START")

	var txProcessed chan struct{}
	serverObj.netSync.QueueBlock(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new blockbeacon END")
}

func (serverObj *Server) OnCrossShard(p *peer.PeerConn,
	msg *wire.MessageCrossShard) {
	Logger.log.Debug("Receive a new crossshard START")

	var txProcessed chan struct{}
	serverObj.netSync.QueueBlock(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new crossshard END")
}

func (serverObj *Server) OnShardToBeacon(p *peer.PeerConn,
	msg *wire.MessageShardToBeacon) {
	Logger.log.Debug("Receive a new shardToBeacon START")

	var txProcessed chan struct{}
	serverObj.netSync.QueueBlock(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new shardToBeacon END")
}

func (serverObj *Server) OnGetBlockBeacon(_ *peer.PeerConn, msg *wire.MessageGetBlockBeacon) {
	Logger.log.Debug("Receive a " + msg.MessageType() + " message START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueGetBlockBeacon(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a " + msg.MessageType() + " message END")
}
func (serverObj *Server) OnGetBlockShard(_ *peer.PeerConn, msg *wire.MessageGetBlockShard) {
	Logger.log.Debug("Receive a " + msg.MessageType() + " message START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueGetBlockShard(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a " + msg.MessageType() + " message END")
}

func (serverObj *Server) OnGetCrossShard(_ *peer.PeerConn, msg *wire.MessageGetCrossShard) {
	Logger.log.Debug("Receive a getcrossshard START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueMessage(nil, msg, txProcessed)
	Logger.log.Debug("Receive a getcrossshard END")
}

func (serverObj *Server) OnGetShardToBeacon(_ *peer.PeerConn, msg *wire.MessageGetShardToBeacon) {
	Logger.log.Debug("Receive a getshardtobeacon START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueMessage(nil, msg, txProcessed)
	Logger.log.Debug("Receive a getshardtobeacon END")
}

// OnTx is invoked when a peer receives a tx message.  It blocks
// until the transaction has been fully processed.  Unlock the block
// handler this does not serialize all transactions through a single thread
// transactions don't rely on the previous one in a linear fashion like blocks.
func (serverObj *Server) OnTx(peer *peer.PeerConn, msg *wire.MessageTx) {
	Logger.log.Debug("Receive a new transaction START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueTx(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new transaction END")
}

func (serverObj *Server) OnTxPrivacyToken(peer *peer.PeerConn, msg *wire.MessageTxPrivacyToken) {
	Logger.log.Debug("Receive a new transaction(privacy token) START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueTxPrivacyToken(nil, msg, txProcessed)
	//<-txProcessed

	Logger.log.Debug("Receive a new transaction(privacy token) END")
}

/*
// OnVersion is invoked when a peer receives a version message
// and is used to negotiate the protocol version details as well as kick start
// the communications.
*/
func (serverObj *Server) OnVersion(peerConn *peer.PeerConn, msg *wire.MessageVersion) {
	Logger.log.Debug("Receive version message START")

	pbk := ""
	pbkType := ""
	if msg.PublicKey != "" {
		//TODO hy set publickey here
		//fmt.Printf("Message %v %v %v\n", msg.SignDataB58, msg.PublicKey, msg.PublicKeyType)
		err := serverObj.consensusEngine.VerifyData([]byte(peerConn.GetListenerPeer().GetPeerID().Pretty()), msg.SignDataB58, msg.PublicKey, msg.PublicKeyType)
		//fmt.Println(err)

		if err == nil {
			pbk = msg.PublicKey
			pbkType = msg.PublicKeyType
		} else {
			peerConn.ForceClose()
			return
		}
	}

	peerConn.GetRemotePeer().SetPublicKey(pbk, pbkType)

	remotePeer := &peer.Peer{}
	remotePeer.SetListeningAddress(msg.LocalAddress)
	remotePeer.SetPeerID(msg.LocalPeerId)
	remotePeer.SetRawAddress(msg.RawLocalAddress)
	remotePeer.SetPublicKey(pbk, pbkType)
	serverObj.cNewPeers <- remotePeer

	if msg.ProtocolVersion != serverObj.protocolVersion {
		Logger.log.Error(errors.New("Not correct version "))
		peerConn.ForceClose()
		return
	}

	// check for accept connection
	if accepted, e := serverObj.connManager.CheckForAcceptConn(peerConn); !accepted {
		// not accept connection -> force close
		Logger.log.Error(e)
		peerConn.ForceClose()
		return
	}

	msgV, err := wire.MakeEmptyMessage(wire.CmdVerack)
	if err != nil {
		return
	}

	msgV.(*wire.MessageVerAck).Valid = true
	msgV.(*wire.MessageVerAck).Timestamp = time.Now()

	peerConn.QueueMessageWithEncoding(msgV, nil, peer.MessageToPeer, nil)

	//	push version message again
	if !peerConn.VerAckReceived() {
		err := serverObj.PushVersionMessage(peerConn)
		if err != nil {
			Logger.log.Error(err)
		}
	}

	Logger.log.Debug("Receive version message END")
}

/*
OnVerAck is invoked when a peer receives a version acknowlege message
*/
func (serverObj *Server) OnVerAck(peerConn *peer.PeerConn, msg *wire.MessageVerAck) {
	Logger.log.Debug("Receive verack message START")

	if msg.Valid {
		peerConn.SetVerValid(true)

		if peerConn.GetIsOutbound() {
			serverObj.addrManager.Good(peerConn.GetRemotePeer())
		}

		// send message for get addr
		//msgSG, err := wire.MakeEmptyMessage(wire.CmdGetAddr)
		//if err != nil {
		//	return
		//}
		//var dc chan<- struct{}
		//peerConn.QueueMessageWithEncoding(msgSG, dc, peer.MessageToPeer, nil)

		//	broadcast addr to all peer
		//listen := serverObj.connManager.GetListeningPeer()
		//msgSA, err := wire.MakeEmptyMessage(wire.CmdAddr)
		//if err != nil {
		//	return
		//}
		//
		//rawPeers := []wire.RawPeer{}
		//peers := serverObj.addrManager.AddressCache()
		//for _, peer := range peers {
		//	getPeerId, _ := serverObj.connManager.GetPeerId(peer.GetRawAddress())
		//	if peerConn.GetRemotePeerID().Pretty() != getPeerId {
		//		pk, pkT := peer.GetPublicKey()
		//		rawPeers = append(rawPeers, wire.RawPeer{peer.GetRawAddress(), pkT, pk})
		//	}
		//}
		//msgSA.(*wire.MessageAddr).RawPeers = rawPeers
		//var doneChan chan<- struct{}
		//listen.GetPeerConnsMtx().Lock()
		//for _, peerConn := range listen.GetPeerConns() {
		//	Logger.log.Debug("QueueMessageWithEncoding", peerConn)
		//	peerConn.QueueMessageWithEncoding(msgSA, doneChan, peer.MessageToPeer, nil)
		//}
		//listen.GetPeerConnsMtx().Unlock()
	} else {
		peerConn.SetVerValid(true)
	}

	Logger.log.Debug("Receive verack message END")
}

func (serverObj *Server) OnGetAddr(peerConn *peer.PeerConn, msg *wire.MessageGetAddr) {
	Logger.log.Debug("Receive getaddr message START")

	// send message for addr
	msgS, err := wire.MakeEmptyMessage(wire.CmdAddr)
	if err != nil {
		return
	}

	peers := serverObj.addrManager.AddressCache()
	rawPeers := []wire.RawPeer{}
	for _, peer := range peers {
		getPeerId, _ := serverObj.connManager.GetPeerId(peer.GetRawAddress())
		if peerConn.GetRemotePeerID().Pretty() != getPeerId {
			pk, pkT := peer.GetPublicKey()
			rawPeers = append(rawPeers, wire.RawPeer{peer.GetRawAddress(), pkT, pk})
		}
	}

	msgS.(*wire.MessageAddr).RawPeers = rawPeers
	var dc chan<- struct{}
	peerConn.QueueMessageWithEncoding(msgS, dc, peer.MessageToPeer, nil)

	Logger.log.Debug("Receive getaddr message END")
}

func (serverObj *Server) OnAddr(peerConn *peer.PeerConn, msg *wire.MessageAddr) {
	Logger.log.Debugf("Receive addr message %v", msg.RawPeers)
}

func (serverObj *Server) OnBFTMsg(p *peer.PeerConn, msg wire.Message) {
	Logger.log.Debug("Receive a BFTMsg START")
	var txProcessed chan struct{}
	isRelayNodeForConsensus := cfg.Accelerator
	if isRelayNodeForConsensus {
		senderPublicKey, _ := p.GetRemotePeer().GetPublicKey()
		// panic(senderPublicKey)
		// fmt.Println("eiiiiiiiiiiiii")
		// os.Exit(0)
		//TODO hy check here
		bestState := serverObj.blockchain.GetBeaconBestState()
		beaconCommitteeList, err := incognitokey.CommitteeKeyListToString(bestState.BeaconCommittee)
		if err != nil {
			panic(err)
		}
		isInBeaconCommittee := common.IndexOfStr(senderPublicKey, beaconCommitteeList) != -1
		if isInBeaconCommittee {
			serverObj.PushMessageToBeacon(msg, map[libp2p.ID]bool{p.GetRemotePeerID(): true})
		}
		shardCommitteeList := make(map[byte][]string)
		for shardID, committee := range bestState.GetShardCommittee() {
			shardCommitteeList[shardID], err = incognitokey.CommitteeKeyListToString(committee)
			if err != nil {
				panic(err)
			}
		}
		for shardID, committees := range shardCommitteeList {
			isInShardCommitee := common.IndexOfStr(senderPublicKey, committees) != -1
			if isInShardCommitee {
				serverObj.PushMessageToShard(msg, shardID, map[libp2p.ID]bool{p.GetRemotePeerID(): true})
				break
			}
		}
	}
	serverObj.netSync.QueueMessage(nil, msg, txProcessed)
	Logger.log.Debug("Receive a BFTMsg END")
}

func (serverObj *Server) OnPeerState(_ *peer.PeerConn, msg *wire.MessagePeerState) {
	Logger.log.Debug("Receive a peerstate START")
	var txProcessed chan struct{}
	serverObj.netSync.QueueMessage(nil, msg, txProcessed)
	Logger.log.Debug("Receive a peerstate END")
}

/*
PushMessageToAll broadcast msg
*/
func (serverObj *Server) PushMessageToAll(msg wire.Message) error {
	Logger.log.Debug("Push msg to all peers")

	// Publish message to highway
	if err := serverObj.highway.PublishMessage(msg); err != nil {
		return err
	}

	return nil
}

/*
PushMessageToPeer push msg to peer
*/
func (serverObj *Server) PushMessageToPeer(msg wire.Message, peerId libp2p.ID) error {
	Logger.log.Debugf("Push msg to peer %s", peerId.Pretty())
	var dc chan<- struct{}
	peerConn := serverObj.connManager.GetConfig().ListenerPeer.GetPeerConnByPeerID(peerId.Pretty())
	if peerConn != nil {
		msg.SetSenderID(serverObj.connManager.GetConfig().ListenerPeer.GetPeerID())
		peerConn.QueueMessageWithEncoding(msg, dc, peer.MessageToPeer, nil)
		Logger.log.Debugf("Pushed peer %s", peerId.Pretty())
		return nil
	} else {
		Logger.log.Error("RemotePeer not exist!")
	}
	return errors.New("RemotePeer not found")
}

/*
PushMessageToPeer push msg to pbk
*/
func (serverObj *Server) PushMessageToPbk(msg wire.Message, pbk string) error {
	Logger.log.Debugf("Push msg to pbk %s", pbk)
	peerConns := serverObj.connManager.GetPeerConnOfPublicKey(pbk)
	if len(peerConns) > 0 {
		for _, peerConn := range peerConns {
			msg.SetSenderID(peerConn.GetListenerPeer().GetPeerID())
			peerConn.QueueMessageWithEncoding(msg, nil, peer.MessageToPeer, nil)
		}
		Logger.log.Debugf("Pushed pbk %s", pbk)
		return nil
	} else {
		Logger.log.Error("RemotePeer not exist!")
	}
	return errors.New("RemotePeer not found")
}

/*
PushMessageToPeer push msg to pbk
*/
func (serverObj *Server) PushMessageToShard(msg wire.Message, shard byte, exclusivePeerIDs map[libp2p.ID]bool) error {
	Logger.log.Debugf("Push msg to shard %d", shard)

	// Publish message to highway
	if err := serverObj.highway.PublishMessageToShard(msg, shard); err != nil {
		return err
	}

	return nil
}

func (serverObj *Server) PushRawBytesToShard(p *peer.PeerConn, msgBytes *[]byte, shard byte) error {
	Logger.log.Debugf("Push raw bytes to shard %d", shard)
	peerConns := serverObj.connManager.GetPeerConnOfShard(shard)
	if len(peerConns) > 0 {
		for _, peerConn := range peerConns {
			if p == nil || peerConn != p {
				peerConn.QueueMessageWithBytes(msgBytes, nil)
			}
		}
		Logger.log.Debugf("Pushed shard %d", shard)
	} else {
		Logger.log.Error("RemotePeer of shard not exist!")
		peerConns := serverObj.connManager.GetPeerConnOfAll()
		for _, peerConn := range peerConns {
			if p == nil || peerConn != p {
				peerConn.QueueMessageWithBytes(msgBytes, nil)
			}
		}
	}
	return nil
}

/*
PushMessageToPeer push msg to beacon node
*/
func (serverObj *Server) PushMessageToBeacon(msg wire.Message, exclusivePeerIDs map[libp2p.ID]bool) error {
	// Publish message to highway
	if err := serverObj.highway.PublishMessage(msg); err != nil {
		return err
	}

	return nil
}

func (serverObj *Server) PushRawBytesToBeacon(p *peer.PeerConn, msgBytes *[]byte) error {
	Logger.log.Debugf("Push raw bytes to beacon")
	peerConns := serverObj.connManager.GetPeerConnOfBeacon()
	if len(peerConns) > 0 {
		for _, peerConn := range peerConns {
			if p == nil || peerConn != p {
				peerConn.QueueMessageWithBytes(msgBytes, nil)
			}
		}
		Logger.log.Debugf("Pushed raw bytes beacon done")
	} else {
		Logger.log.Error("RemotePeer of beacon raw bytes not exist!")
		peerConns := serverObj.connManager.GetPeerConnOfAll()
		for _, peerConn := range peerConns {
			if p == nil || peerConn != p {
				peerConn.QueueMessageWithBytes(msgBytes, nil)
			}
		}
	}
	return nil
}

// handleAddPeerMsg deals with adding new peers.  It is invoked from the
// peerHandler goroutine.
func (serverObj *Server) handleAddPeerMsg(peer *peer.Peer) bool {
	if peer == nil {
		return false
	}
	Logger.log.Debug("Zero peer have just sent a message version")
	//Logger.log.Debug(peer)
	return true
}

func (serverObj *Server) PushVersionMessage(peerConn *peer.PeerConn) error {
	// push message version
	msg, err := wire.MakeEmptyMessage(wire.CmdVersion)
	msg.(*wire.MessageVersion).Timestamp = time.Now().UnixNano()
	msg.(*wire.MessageVersion).LocalAddress = peerConn.GetListenerPeer().GetListeningAddress()
	msg.(*wire.MessageVersion).RawLocalAddress = peerConn.GetListenerPeer().GetRawAddress()
	msg.(*wire.MessageVersion).LocalPeerId = peerConn.GetListenerPeer().GetPeerID()
	msg.(*wire.MessageVersion).RemoteAddress = peerConn.GetListenerPeer().GetListeningAddress()
	msg.(*wire.MessageVersion).RawRemoteAddress = peerConn.GetListenerPeer().GetRawAddress()
	msg.(*wire.MessageVersion).RemotePeerId = peerConn.GetListenerPeer().GetPeerID()
	msg.(*wire.MessageVersion).ProtocolVersion = serverObj.protocolVersion

	// ValidateTransaction Public Key from ProducerPrvKey
	// publicKeyInBase58CheckEncode, publicKeyType := peerConn.GetListenerPeer().GetConfig().ConsensusEngine.GetCurrentMiningPublicKey()
	signDataInBase58CheckEncode := common.EmptyString
	// if publicKeyInBase58CheckEncode != "" {
	// msg.(*wire.MessageVersion).PublicKey = publicKeyInBase58CheckEncode
	// msg.(*wire.MessageVersion).PublicKeyType = publicKeyType
	// Logger.log.Info("Start Process Discover Peers", publicKeyInBase58CheckEncode)
	// sign data
	msg.(*wire.MessageVersion).PublicKey, msg.(*wire.MessageVersion).PublicKeyType, signDataInBase58CheckEncode, err = peerConn.GetListenerPeer().GetConfig().ConsensusEngine.SignDataWithCurrentMiningKey([]byte(peerConn.GetRemotePeer().GetPeerID().Pretty()))
	if err == nil {
		msg.(*wire.MessageVersion).SignDataB58 = signDataInBase58CheckEncode
	}
	// }
	// if peerConn.GetListenerPeer().GetConfig().UserKeySet != nil {
	// 	msg.(*wire.MessageVersion).PublicKey = peerConn.GetListenerPeer().GetConfig().UserKeySet.GetPublicKeyInBase58CheckEncode()
	// 	signDataB58, err := peerConn.GetListenerPeer().GetConfig().UserKeySet.SignDataInBase58CheckEncode()
	// 	if err == nil {
	// 		msg.(*wire.MessageVersion).SignDataB58 = signDataB58
	// 	}
	// }
	if err != nil {
		return err
	}
	peerConn.QueueMessageWithEncoding(msg, nil, peer.MessageToPeer, nil)
	return nil
}

func (serverObj *Server) PushMessageGetBlockBeaconByHeight(from uint64, to uint64) error {
	msgs, err := serverObj.highway.Requester.GetBlockBeaconByHeight(
		false, // bySpecific
		from,  // from
		nil,   // heights (this params just != nil if bySpecific == true)
		to,    // to
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}
	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockBeaconBySpecificHeight(heights []uint64, getFromPool bool) error {
	Logger.log.Infof("[byspecific] Get blk beacon by Specific heights %v", heights)
	msgs, err := serverObj.highway.Requester.GetBlockBeaconByHeight(
		true,    // bySpecific
		0,       // from
		heights, // heights (this params just != nil if bySpecific == true)
		0,       // to
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}
	// TODO(@0xbunyip): instead of putting response to queue, use it immediately in synker
	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockBeaconByHash(blkHashes []common.Hash, getFromPool bool, peerID libp2p.ID) error {
	msgs, err := serverObj.highway.Requester.GetBlockBeaconByHash(
		blkHashes, // by blockHashes
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}
	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockShardByHeight(shardID byte, from uint64, to uint64) error {
	msgs, err := serverObj.highway.Requester.GetBlockShardByHeight(
		int32(shardID), // shardID
		false,          // bySpecific
		from,           // from
		nil,            // heights
		to,             // to
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}

	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockShardBySpecificHeight(shardID byte, heights []uint64, getFromPool bool) error {
	Logger.log.Infof("[byspecific] Get blk shard %v by Specific heights %v", shardID, heights)
	msgs, err := serverObj.highway.Requester.GetBlockShardByHeight(
		int32(shardID), // shardID
		true,           // bySpecific
		0,              // from
		heights,        // heights
		0,              // to
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}
	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockShardByHash(shardID byte, blkHashes []common.Hash, getFromPool bool, peerID libp2p.ID) error {
	Logger.log.Infof("[blkbyhash] Get blk shard by hash %v", blkHashes)
	msgs, err := serverObj.highway.Requester.GetBlockShardByHash(
		int32(shardID),
		blkHashes, // by blockHashes
	)
	if err != nil {
		Logger.log.Infof("[blkbyhash] Get blk shard by hash error %v ", err)
		Logger.log.Error(err)
		return err
	}
	Logger.log.Infof("[blkbyhash] Get blk shard by hash get %v ", msgs)

	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockShardToBeaconByHeight(shardID byte, from uint64, to uint64) error {
	msgs, err := serverObj.highway.Requester.GetBlockShardToBeaconByHeight(
		int32(shardID),
		false, // by Specific
		from,  // sfrom
		nil,   // nil because request via [from:to]
		to,    // to
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}

	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PushMessageGetBlockShardToBeaconByHash(shardID byte, blkHashes []common.Hash, getFromPool bool, peerID libp2p.ID) error {
	Logger.log.Debugf("Send a GetShardToBeacon")
	listener := serverObj.connManager.GetConfig().ListenerPeer
	msg, err := wire.MakeEmptyMessage(wire.CmdGetShardToBeacon)
	if err != nil {
		return err
	}
	msg.(*wire.MessageGetShardToBeacon).ByHash = true
	msg.(*wire.MessageGetShardToBeacon).FromPool = getFromPool
	msg.(*wire.MessageGetShardToBeacon).ShardID = shardID
	msg.(*wire.MessageGetShardToBeacon).BlkHashes = blkHashes
	msg.(*wire.MessageGetShardToBeacon).Timestamp = time.Now().Unix()
	msg.SetSenderID(listener.GetPeerID())
	Logger.log.Debugf("Send a GetCrossShard from %s", listener.GetRawAddress())
	if peerID == "" {
		return serverObj.PushMessageToShard(msg, shardID, map[libp2p.ID]bool{})
	}
	return serverObj.PushMessageToPeer(msg, peerID)
}

func (serverObj *Server) PushMessageGetBlockShardToBeaconBySpecificHeight(
	shardID byte,
	blkHeights []uint64,
	getFromPool bool,
	peerID libp2p.ID,
) error {
	msgs, err := serverObj.highway.Requester.GetBlockShardToBeaconByHeight(
		int32(shardID),
		true,       //by Specific
		0,          //from 0 to 0 because request via blkheights
		blkHeights, //
		0,          // to 0
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}

	serverObj.putResponseMsgs(msgs)
	return nil

}

func (serverObj *Server) PushMessageGetBlockCrossShardByHash(fromShard byte, toShard byte, blkHashes []common.Hash, getFromPool bool, peerID libp2p.ID) error {
	Logger.log.Debugf("Send a GetCrossShard")
	listener := serverObj.connManager.GetConfig().ListenerPeer
	msg, err := wire.MakeEmptyMessage(wire.CmdGetCrossShard)
	if err != nil {
		return err
	}
	msg.(*wire.MessageGetCrossShard).ByHash = true
	msg.(*wire.MessageGetCrossShard).FromPool = getFromPool
	msg.(*wire.MessageGetCrossShard).FromShardID = fromShard
	msg.(*wire.MessageGetCrossShard).ToShardID = toShard
	msg.(*wire.MessageGetCrossShard).BlkHashes = blkHashes
	msg.(*wire.MessageGetCrossShard).Timestamp = time.Now().Unix()
	msg.SetSenderID(listener.GetPeerID())
	Logger.log.Debugf("Send a GetCrossShard from %s", listener.GetRawAddress())
	if peerID == "" {
		return serverObj.PushMessageToShard(msg, fromShard, map[libp2p.ID]bool{})
	}
	return serverObj.PushMessageToPeer(msg, peerID)

}

func (serverObj *Server) PushMessageGetBlockCrossShardBySpecificHeight(fromShard byte, toShard byte, blkHeights []uint64, getFromPool bool, peerID libp2p.ID) error {
	msgs, err := serverObj.highway.Requester.GetBlockCrossShardByHeight(
		int32(fromShard),
		int32(toShard),
		blkHeights,
		getFromPool,
	)
	if err != nil {
		Logger.log.Error(err)
		return err
	}

	serverObj.putResponseMsgs(msgs)
	return nil
}

func (serverObj *Server) PublishNodeState(userLayer string, shardID int) error {
	Logger.log.Infof("[peerstate] Start Publish SelfPeerState")
	listener := serverObj.connManager.GetConfig().ListenerPeer

	// if (userRole != common.CommitteeRole) && (userRole != common.ValidatorRole) && (userRole != common.ProposerRole) {
	// 	return errors.New("Not in committee, don't need to publish node state!")
	// }

	userKey, _ := serverObj.consensusEngine.GetCurrentMiningPublicKey()
	metrics.SetGlobalParam("MINING_PUBKEY", userKey)
	msg, err := wire.MakeEmptyMessage(wire.CmdPeerState)
	if err != nil {
		return err
	}
	msg.(*wire.MessagePeerState).Beacon = blockchain_v2.ChainState{
		serverObj.blockChain.BestState.Beacon.BestBlock.Header.Timestamp,
		serverObj.blockChain.BestState.Beacon.BeaconHeight,
		serverObj.blockChain.BestState.Beacon.BestBlockHash,
		serverObj.blockChain.BestState.Beacon.Hash(),
	}

	if userLayer != common.BeaconRole {
		msg.(*wire.MessagePeerState).Shards[byte(shardID)] = blockchain_v2.ChainState{
			serverObj.blockChain.BestState.Shard[byte(shardID)].BestBlock.Header.Timestamp,
			serverObj.blockChain.BestState.Shard[byte(shardID)].ShardHeight,
			serverObj.blockChain.BestState.Shard[byte(shardID)].BestBlockHash,
			serverObj.blockChain.BestState.Shard[byte(shardID)].Hash(),
		}
	} else {
		msg.(*wire.MessagePeerState).ShardToBeaconPool = serverObj.shardToBeaconPool.GetAllBlockHeight()
		Logger.log.Infof("[peerstate] %v", msg.(*wire.MessagePeerState).ShardToBeaconPool)
	}

	if (cfg.NodeMode == common.NodeModeAuto || cfg.NodeMode == common.NodeModeShard) && shardID >= 0 {
		msg.(*wire.MessagePeerState).CrossShardPool[byte(shardID)] = serverObj.crossShardPool[byte(shardID)].GetValidBlockHeight()
	}

	//
	currentMiningKey := serverObj.consensusEngine.GetMiningPublicKeys()[serverObj.blockChain.BestState.Beacon.ConsensusAlgorithm]
	msg.(*wire.MessagePeerState).SenderMiningPublicKey, err = currentMiningKey.ToBase58()
	if err != nil {
		return err
	}
	msg.SetSenderID(serverObj.highway.LocalHost.Host.ID())
	Logger.log.Infof("[peerstate] PeerID send to Proxy when publish node state %v \n", listener.GetPeerID())
	if err != nil {
		return err
	}
	Logger.log.Debugf("Publish peerstate")
	serverObj.PushMessageToAll(msg)
	return nil
}

func (serverObj *Server) BoardcastNodeState() error {
	listener := serverObj.connManager.GetConfig().ListenerPeer
	msg, err := wire.MakeEmptyMessage(wire.CmdPeerState)
	if err != nil {
		return err
	}
	msg.(*wire.MessagePeerState).Beacon = blockchain_v2.ChainState{
		serverObj.blockChain.BestState.Beacon.BestBlock.Header.Timestamp,
		serverObj.blockChain.BestState.Beacon.BeaconHeight,
		serverObj.blockChain.BestState.Beacon.BestBlockHash,
		serverObj.blockChain.BestState.Beacon.Hash(),
	}
	for _, shardID := range serverObj.blockChain.Synker.GetCurrentSyncShards() {
		msg.(*wire.MessagePeerState).Shards[shardID] = blockchain_v2.ChainState{
			serverObj.blockChain.BestState.Shard[shardID].BestBlock.Header.Timestamp,
			serverObj.blockChain.BestState.Shard[shardID].ShardHeight,
			serverObj.blockChain.BestState.Shard[shardID].BestBlockHash,
			serverObj.blockChain.BestState.Shard[shardID].Hash(),
		}
	}
	msg.(*wire.MessagePeerState).ShardToBeaconPool = serverObj.shardToBeaconPool.GetValidBlockHeight()

	publicKeyInBase58CheckEncode, _ := serverObj.consensusEngine.GetCurrentMiningPublicKey()
	// signDataInBase58CheckEncode := common.EmptyString
	if publicKeyInBase58CheckEncode != "" {
		_, shardID := serverObj.consensusEngine.GetUserLayer()
		if (cfg.NodeMode == common.NodeModeAuto || cfg.NodeMode == common.NodeModeShard) && shardID >= 0 {
			msg.(*wire.MessagePeerState).CrossShardPool[byte(shardID)] = serverObj.crossShardPool[byte(shardID)].GetValidBlockHeight()
		}
	}
	userKey, _ := serverObj.consensusEngine.GetCurrentMiningPublicKey()
	if userKey != "" {
		metrics.SetGlobalParam("MINING_PUBKEY", userKey)
		userRole, shardID := serverObj.blockChain.BestState.Beacon.GetPubkeyRole(userKey, serverObj.blockChain.BestState.Beacon.BestBlock.Header.Round)
		if (cfg.NodeMode == common.NodeModeAuto || cfg.NodeMode == common.NodeModeShard) && userRole == common.NodeModeShard {
			userRole = serverObj.blockChain.BestState.Shard[shardID].GetPubkeyRole(userKey, serverObj.blockChain.BestState.Shard[shardID].BestBlock.Header.Round)
			if userRole == "shard-proposer" || userRole == "shard-validator" {
				msg.(*wire.MessagePeerState).CrossShardPool[shardID] = serverObj.crossShardPool[shardID].GetValidBlockHeight()
			}
		}
	}
	msg.SetSenderID(listener.GetPeerID())
	Logger.log.Debugf("Broadcast peerstate from %s", listener.GetRawAddress())
	serverObj.PushMessageToAll(msg)
	return nil
}

func (serverObj *Server) PushMessageToChain(msg wire.Message, chain blockchain_v2.ChainInterface) error {
	chainID := chain.GetShardID()
	if chainID == -1 {
		serverObj.PushMessageToBeacon(msg, map[libp2p.ID]bool{})
	} else {
		serverObj.PushMessageToShard(msg, byte(chainID), map[libp2p.ID]bool{})
	}
	return nil
}
func (serverObj *Server) PushBlockToAll(block blockinterface.BlockInterface, isBeacon bool) error {
	if isBeacon {
		msg, err := wire.MakeEmptyMessage(wire.CmdBlockBeacon)
		if err != nil {
			Logger.log.Error(err)
			return err
		}
		// msg.(*wire.MessageBlockBeacon).Block = block
		serverObj.PushMessageToAll(msg)
		return nil
	} else {
		shardBlock := block
		msgShard, err := wire.MakeEmptyMessage(wire.CmdBlockShard)
		if err != nil {
			Logger.log.Error(err)
			return err
		}
		// msgShard.(*wire.MessageBlockShard).Block = shardBlock
		serverObj.PushMessageToShard(msgShard, shardBlock.Header.ShardID, map[libp2p.ID]bool{})

		shardToBeaconBlk := shardBlock.CreateShardToBeaconBlock(serverObj.blockChain)
		msgShardToBeacon, err := wire.MakeEmptyMessage(wire.CmdBlkShardToBeacon)
		if err != nil {
			Logger.log.Error(err)
			return err
		}
		msgShardToBeacon.(*wire.MessageShardToBeacon).Block = shardToBeaconBlk
		serverObj.PushMessageToBeacon(msgShardToBeacon, map[libp2p.ID]bool{})
		crossShardBlks := shardBlock.CreateAllCrossShardBlock(serverObj.blockChain.BestState.Beacon.ActiveShards)
		for shardID, crossShardBlk := range crossShardBlks {
			msgCrossShardShard, err := wire.MakeEmptyMessage(wire.CmdCrossShard)
			if err != nil {
				Logger.log.Error(err)
				return err
			}
			msgCrossShardShard.(*wire.MessageCrossShard).Block = crossShardBlk
			serverObj.PushMessageToShard(msgCrossShardShard, shardID, map[libp2p.ID]bool{})
		}
	}
	return nil
}

func (serverObj *Server) TransactionPoolBroadcastLoop() {
	ticker := time.NewTicker(serverObj.memPool.ScanTime)
	defer ticker.Stop()
	for _ = range ticker.C {
		txDescs := serverObj.memPool.GetPool()
		for _, txDesc := range txDescs {
			<-time.Tick(50 * time.Millisecond)
			if !txDesc.IsFowardMessage {
				tx := txDesc.Desc.Tx
				switch tx.GetType() {
				case common.TxNormalType:
					{
						txMsg, err := wire.MakeEmptyMessage(wire.CmdTx)
						if err != nil {
							continue
						}
						normalTx := tx.(*transaction.Tx)
						txMsg.(*wire.MessageTx).Transaction = normalTx
						err = serverObj.PushMessageToAll(txMsg)
						if err == nil {
							serverObj.memPool.MarkForwardedTransaction(*tx.Hash())
						}
					}
				case common.TxCustomTokenPrivacyType:
					{
						txMsg, err := wire.MakeEmptyMessage(wire.CmdPrivacyCustomToken)
						if err != nil {
							continue
						}
						customPrivacyTokenTx := tx.(*transaction.TxCustomTokenPrivacy)
						txMsg.(*wire.MessageTxPrivacyToken).Transaction = customPrivacyTokenTx
						err = serverObj.PushMessageToAll(txMsg)
						if err == nil {
							serverObj.memPool.MarkForwardedTransaction(*tx.Hash())
						}
					}
				}
			}
		}
	}
}
