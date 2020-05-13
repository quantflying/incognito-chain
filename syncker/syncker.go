package syncker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/wire"
)

type SynckerManagerConfig struct {
	Node       Server
	Blockchain *blockchain.BlockChain
}

type SynckerManager struct {
	isEnabled             bool //0 > stop, 1: running
	config                *SynckerManagerConfig
	BeaconSyncProcess     *BeaconSyncProcess
	S2BSyncProcess        *S2BSyncProcess
	ShardSyncProcess      map[int]*ShardSyncProcess
	CrossShardSyncProcess map[int]*CrossShardSyncProcess
	beaconPool            *BlkPool
	shardPool             map[int]*BlkPool
	s2bPool               *BlkPool
	crossShardPool        map[int]*BlkPool
}

func NewSynckerManager() *SynckerManager {
	s := &SynckerManager{
		ShardSyncProcess:      make(map[int]*ShardSyncProcess),
		shardPool:             make(map[int]*BlkPool),
		CrossShardSyncProcess: make(map[int]*CrossShardSyncProcess),
		crossShardPool:        make(map[int]*BlkPool),
	}
	return s
}

func (synckerManager *SynckerManager) Init(config *SynckerManagerConfig) {
	synckerManager.config = config
	//init beacon sync process
	synckerManager.BeaconSyncProcess = NewBeaconSyncProcess(synckerManager.config.Node, synckerManager.config.Blockchain.BeaconChain)
	synckerManager.S2BSyncProcess = synckerManager.BeaconSyncProcess.s2bSyncProcess
	synckerManager.beaconPool = synckerManager.BeaconSyncProcess.beaconPool
	synckerManager.s2bPool = synckerManager.S2BSyncProcess.s2bPool

	//init shard sync process
	for _, chain := range synckerManager.config.Blockchain.ShardChain {
		sid := chain.GetShardID()
		synckerManager.ShardSyncProcess[sid] = NewShardSyncProcess(sid, synckerManager.config.Node, synckerManager.config.Blockchain.BeaconChain, chain)
		synckerManager.shardPool[sid] = synckerManager.ShardSyncProcess[sid].shardPool
		synckerManager.CrossShardSyncProcess[sid] = synckerManager.ShardSyncProcess[sid].crossShardSyncProcess
		synckerManager.crossShardPool[sid] = synckerManager.CrossShardSyncProcess[sid].crossShardPool

	}

	//watch commitee change
	go synckerManager.manageSyncProcess()

	//Publish node state to other peer
	go func() {
		t := time.NewTicker(time.Second * 3)
		for _ = range t.C {
			_, chainID := synckerManager.config.Node.GetUserMiningState()
			if chainID == -1 {
				_ = synckerManager.config.Node.PublishNodeState("beacon", chainID)
			}
			if chainID >= 0 {
				_ = synckerManager.config.Node.PublishNodeState("shard", chainID)
			}
		}
	}()
}

func (synckerManager *SynckerManager) Start() {
	synckerManager.isEnabled = true
}

func (synckerManager *SynckerManager) Stop() {
	synckerManager.isEnabled = false
	synckerManager.BeaconSyncProcess.stop()
	for _, chain := range synckerManager.ShardSyncProcess {
		chain.stop()
	}
}

// periodically check user commmittee status, enable shard sync process if needed (beacon always start)
func (synckerManager *SynckerManager) manageSyncProcess() {
	defer time.AfterFunc(time.Second*5, synckerManager.manageSyncProcess)

	//check if enable
	if !synckerManager.isEnabled || synckerManager.config == nil {
		return
	}
	role, chainID := synckerManager.config.Node.GetUserMiningState()
	synckerManager.BeaconSyncProcess.isCommittee = (role == common.CommitteeRole) && (chainID == -1)
	synckerManager.BeaconSyncProcess.start()
	wantedShard := synckerManager.config.Blockchain.GetWantedShard()
	for sid, syncProc := range synckerManager.ShardSyncProcess {
		if _, ok := wantedShard[byte(sid)]; ok || (int(sid) == chainID) {
			syncProc.start()
		} else {
			syncProc.stop()
		}
		syncProc.isCommittee = role == common.CommitteeRole || role == common.PendingRole
	}

}

//Process incomming broadcast block
func (synckerManager *SynckerManager) ReceiveBlock(blk interface{}, peerID string) {
	switch blk.(type) {
	case *blockchain.BeaconBlock:
		beaconBlk := blk.(*blockchain.BeaconBlock)
		fmt.Printf("syncker: receive beacon block %d \n", beaconBlk.GetHeight())
		//create fake s2b pool peerstate
		if synckerManager.BeaconSyncProcess != nil {
			synckerManager.beaconPool.AddBlock(beaconBlk)
			synckerManager.BeaconSyncProcess.beaconPeerStateCh <- &wire.MessagePeerState{
				Beacon: wire.ChainState{
					Timestamp: beaconBlk.Header.Timestamp,
					BlockHash: *beaconBlk.Hash(),
					Height:    beaconBlk.GetHeight(),
				},
				SenderID:  peerID,
				Timestamp: time.Now().Unix(),
			}
		}

	case *blockchain.ShardBlock:

		shardBlk := blk.(*blockchain.ShardBlock)
		//fmt.Printf("syncker: receive shard block %d \n", shardBlk.GetHeight())
		if synckerManager.shardPool[shardBlk.GetShardID()] != nil {
			synckerManager.shardPool[shardBlk.GetShardID()].AddBlock(shardBlk)
		}

	case *blockchain.ShardToBeaconBlock:
		s2bBlk := blk.(*blockchain.ShardToBeaconBlock)
		if synckerManager.S2BSyncProcess != nil {
			synckerManager.s2bPool.AddBlock(s2bBlk)
			//fmt.Println("syncker AddBlock S2B", s2bBlk.Header.ShardID, s2bBlk.Header.Height)
			//create fake s2b pool peerstate
			synckerManager.S2BSyncProcess.s2bPeerStateCh <- &wire.MessagePeerState{
				SenderID:          peerID,
				ShardToBeaconPool: map[byte][]uint64{s2bBlk.Header.ShardID: []uint64{1, s2bBlk.GetHeight()}},
				Timestamp:         time.Now().Unix(),
			}
		}

	case *blockchain.CrossShardBlock:
		csBlk := blk.(*blockchain.CrossShardBlock)
		if synckerManager.CrossShardSyncProcess[int(csBlk.ToShardID)] != nil {
			fmt.Printf("crossdebug: receive block from %d to %d (%synckerManager)\n", csBlk.Header.ShardID, csBlk.ToShardID, csBlk.Hash().String())
			synckerManager.crossShardPool[int(csBlk.ToShardID)].AddBlock(csBlk)
		}
	}
}

//Process incomming broadcast peerstate
func (synckerManager *SynckerManager) ReceivePeerState(peerState *wire.MessagePeerState) {
	//b, _ := json.Marshal(peerState)
	//fmt.Println("SYNCKER: receive peer state", string(b))
	//beacon
	if peerState.Beacon.Height != 0 && synckerManager.BeaconSyncProcess != nil {
		synckerManager.BeaconSyncProcess.beaconPeerStateCh <- peerState
	}
	//s2b
	if len(peerState.ShardToBeaconPool) != 0 && synckerManager.S2BSyncProcess != nil {
		synckerManager.S2BSyncProcess.s2bPeerStateCh <- peerState
	}
	//shard
	for sid, _ := range peerState.Shards {
		if synckerManager.ShardSyncProcess[int(sid)] != nil {
			synckerManager.ShardSyncProcess[int(sid)].shardPeerStateCh <- peerState
		}

	}
}

//Get S2B Block for creating beacon block
func (synckerManager *SynckerManager) GetS2BBlocksForBeaconProducer(bestViewShardHash map[byte]common.Hash) map[byte][]interface{} {
	res := make(map[byte][]interface{})

	for i := 0; i < synckerManager.config.Node.GetChainParam().ActiveShards; i++ {
		v := bestViewShardHash[byte(i)]
		//beacon beststate dont have shard hash  => create one
		if (&v).IsEqual(&common.Hash{}) {
			blk := *synckerManager.config.Node.GetChainParam().GenesisShardBlock
			blk.Header.ShardID = byte(i)
			v = *blk.Hash()
		}

		for _, v := range synckerManager.s2bPool.GetFinalBlockFromBlockHash(v.String()) {
			res[byte(i)] = append(res[byte(i)], v)
			//fmt.Println("syncker: get block ", i, v.GetHeight(), v.Hash().String())
		}
	}
	return res
}

//Get Crossshard Block for creating shardblock block
func (synckerManager *SynckerManager) GetCrossShardBlocksForShardProducer(toShard byte) map[byte][]interface{} {
	//get last confirm crossshard -> process request until retrieve info
	res := make(map[byte][]interface{})
	beaconDB := synckerManager.config.Node.GetBeaconChainDatabase()
	lastRequestCrossShard := synckerManager.ShardSyncProcess[int(toShard)].Chain.GetCrossShardState()
	for i := 0; i < synckerManager.config.Node.GetChainParam().ActiveShards; i++ {
		for {
			if i == int(toShard) {
				break
			}
			requestHeight := lastRequestCrossShard[byte(i)]
			nextCrossShardInfo := synckerManager.config.Node.FetchNextCrossShard(i, int(toShard), requestHeight)
			//Logger.Info("nextCrossShardInfo.NextCrossShardHeight", i, toShard, requestHeight, nextCrossShardInfo)
			if nextCrossShardInfo == nil {
				break
			}
			beaconHash, _ := common.Hash{}.NewHashFromStr(nextCrossShardInfo.ConfirmBeaconHash)
			beaconBlockBytes, err := rawdbv2.GetBeaconBlockByHash(beaconDB, *beaconHash)
			if err != nil {
				break
			}

			beaconBlock := new(blockchain.BeaconBlock)
			json.Unmarshal(beaconBlockBytes, beaconBlock)
			for _, shardState := range beaconBlock.Body.ShardState[byte(i)] {
				if shardState.Height == nextCrossShardInfo.NextCrossShardHeight {
					if synckerManager.crossShardPool[int(toShard)].HasHash(shardState.Hash) {
						res[byte(i)] = append(res[byte(i)], synckerManager.crossShardPool[int(toShard)].GetBlock(shardState.Hash))
					}
					lastRequestCrossShard[byte(i)] = nextCrossShardInfo.NextCrossShardHeight
					break
				}
			}
		}
	}
	return res
}

//Get S2B Block for validating beacon block
func (synckerManager *SynckerManager) GetS2BBlocksForBeaconValidator(bestViewShardHash map[byte]common.Hash, list map[byte][]common.Hash) (map[byte][]interface{}, error) {
	s2bPoolLists := synckerManager.GetS2BBlocksForBeaconProducer(bestViewShardHash)

	missingBlocks := compareLists(s2bPoolLists, list)
	// synckerManager.config.Server.
	if len(missingBlocks) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		synckerManager.StreamMissingShardToBeaconBlock(ctx, missingBlocks)
		fmt.Println("debug finish stream missing s2b block")

		s2bPoolLists = synckerManager.GetS2BBlocksForBeaconProducer(bestViewShardHash)
		missingBlocks = compareLists(s2bPoolLists, list)
		if len(missingBlocks) > 0 {
			return nil, errors.New("Unable to sync required block in time")
		}
	}

	return s2bPoolLists, nil
}

//Stream Missing ShardToBeacon Block
func (synckerManager *SynckerManager) StreamMissingShardToBeaconBlock(ctx context.Context, missingBlock map[byte][]common.Hash) {
	fmt.Println("debug stream missing s2b block", missingBlock)
	wg := sync.WaitGroup{}
	for i, v := range missingBlock {
		wg.Add(1)
		go func(sid byte, list []common.Hash) {
			defer wg.Done()
			hashes := [][]byte{}
			for _, h := range list {
				hashes = append(hashes, h.Bytes())
			}
			ch, err := synckerManager.config.Node.RequestShardToBeaconBlocksByHashViaStream(ctx, "", int(sid), hashes)
			if err != nil {
				fmt.Println("Syncker: create channel fail")
				return
			}
			//receive
			for {
				select {
				case blk := <-ch:
					if !isNil(blk) {
						synckerManager.s2bPool.AddBlock(blk.(common.BlockPoolInterface))
					} else {
						return
					}
				}
			}
		}(i, v)
	}
	wg.Wait()
}

//Get Crossshard Block for validating shardblock block
func (synckerManager *SynckerManager) GetCrossShardBlocksForShardValidator(toShard byte, list map[byte][]uint64) (map[byte][]interface{}, error) {
	crossShardPoolLists := synckerManager.GetCrossShardBlocksForShardProducer(toShard)

	missingBlocks := compareListsByHeight(crossShardPoolLists, list)
	// synckerManager.config.Server.
	if len(missingBlocks) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		synckerManager.StreamMissingCrossShardBlock(ctx, toShard, missingBlocks)
		//Logger.Info("debug finish stream missing crossX block")

		crossShardPoolLists = synckerManager.GetCrossShardBlocksForShardProducer(toShard)
		//Logger.Info("get crosshshard block for shard producer", crossShardPoolLists)
		missingBlocks = compareListsByHeight(crossShardPoolLists, list)

		if len(missingBlocks) > 0 {
			return nil, errors.New("Unable to sync required block in time")
		}
	}
	return crossShardPoolLists, nil
}

//Stream Missing CrossShard Block
func (synckerManager *SynckerManager) StreamMissingCrossShardBlock(ctx context.Context, toShard byte, missingBlock map[byte][]uint64) {
	for fromShard, missingHeight := range missingBlock {
		//fmt.Println("debug stream missing crossshard block", int(fromShard), int(toShard), missingHeight)
		ch, err := synckerManager.config.Node.RequestCrossShardBlocksViaStream(ctx, "", int(fromShard), int(toShard), missingHeight)
		if err != nil {
			fmt.Println("Syncker: create channel fail")
			return
		}
		//receive
		for {
			select {
			case blk := <-ch:
				if !isNil(blk) {
					Logger.Infof("Receive crosshard block from shard %v ->  %v, hash %v", fromShard, toShard, blk.(common.BlockPoolInterface).Hash().String())
					synckerManager.crossShardPool[int(toShard)].AddBlock(blk.(common.BlockPoolInterface))
				} else {
					//Logger.Info("Block is nil, break stream")
					return
				}
			}
		}
	}
}

////Get Crossshard Block for validating shardblock block
//func (synckerManager *SynckerManager) GetCrossShardBlocksForShardValidatorByHash(toShard byte, list map[byte][]common.Hash) (map[byte][]interface{}, error) {
//	crossShardPoolLists := synckerManager.GetCrossShardBlocksForShardProducer(toShard)
//
//	missingBlocks := compareLists(crossShardPoolLists, list)
//	// synckerManager.config.Server.
//	if len(missingBlocks) > 0 {
//		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
//		synckerManager.StreamMissingCrossShardBlockByHash(ctx, toShard, missingBlocks)
//		fmt.Println("debug finish stream missing s2b block")
//
//		crossShardPoolLists = synckerManager.GetCrossShardBlocksForShardProducer(toShard)
//		missingBlocks = compareLists(crossShardPoolLists, list)
//
//		if len(missingBlocks) > 0 {
//			return nil, errors.New("Unable to sync required block in time")
//		}
//	}
//	return crossShardPoolLists, nil
//}
//
////Stream Missing CrossShard Block
//func (synckerManager *SynckerManager) StreamMissingCrossShardBlockByHash(ctx context.Context, toShard byte, missingBlock map[byte][]common.Hash) {
//	fmt.Println("debug stream missing crossshard block", missingBlock)
//	wg := sync.WaitGroup{}
//	for i, v := range missingBlock {
//		wg.Add(1)
//		go func(sid byte, list []common.Hash) {
//			defer wg.Done()
//			hashes := [][]byte{}
//			for _, h := range list {
//				hashes = append(hashes, h.Bytes())
//			}
//			ch, err := synckerManager.config.Node.RequestCrossShardBlocksByHashViaStream(ctx, "", int(sid), int(toShard), hashes)
//			if err != nil {
//				fmt.Println("Syncker: create channel fail")
//				return
//			}
//			//receive
//			for {
//				select {
//				case blk := <-ch:
//					if !isNil(blk) {
//						synckerManager.crossShardPool[int(toShard)].AddBlock(blk.(common.BlockPoolInterface))
//					} else {
//						return
//					}
//				}
//			}
//		}(i, v)
//	}
//	wg.Wait()
//}

//Sync missing beacon block  from a hash to our final view (skip if we already have)
func (synckerManager *SynckerManager) SyncMissingBeaconBlock(ctx context.Context, peerID string, fromHash common.Hash) {
	requestHash := fromHash
	for {
		ch, err := synckerManager.config.Node.RequestBeaconBlocksByHashViaStream(ctx, peerID, [][]byte{requestHash.Bytes()})
		if err != nil {
			fmt.Println("Syncker: create channel fail")
			return
		}
		blk := <-ch
		if !isNil(blk) {
			if blk.(*blockchain.BeaconBlock).GetHeight() <= synckerManager.config.Blockchain.BeaconChain.GetFinalViewHeight() {
				return
			}
			synckerManager.beaconPool.AddBlock(blk.(common.BlockPoolInterface))
			prevHash := blk.(*blockchain.BeaconBlock).GetPrevHash()
			if v := synckerManager.config.Blockchain.BeaconChain.GetViewByHash(prevHash); v == nil {
				requestHash = prevHash
				continue
			}
		}
		return
	}
}

//Sync back missing shard block from a hash to our final views (skip if we already have)
func (synckerManager *SynckerManager) SyncMissingShardBlock(ctx context.Context, peerID string, sid byte, fromHash common.Hash) {
	requestHash := fromHash
	for {
		ch, err := synckerManager.config.Node.RequestShardBlocksByHashViaStream(ctx, peerID, int(sid), [][]byte{requestHash.Bytes()})
		if err != nil {
			fmt.Println("Syncker: create channel fail")
			return
		}
		blk := <-ch
		if !isNil(blk) {
			if blk.(*blockchain.ShardBlock).GetHeight() <= synckerManager.config.Blockchain.ShardChain[sid].GetFinalViewHeight() {
				return
			}
			synckerManager.shardPool[int(sid)].AddBlock(blk.(common.BlockPoolInterface))
			prevHash := blk.(*blockchain.ShardBlock).GetPrevHash()
			if v := synckerManager.config.Blockchain.ShardChain[sid].GetViewByHash(prevHash); v == nil {
				requestHash = prevHash
				continue
			}
		}
		return

	}
}

//Get Status Function
type syncInfo struct {
	IsSync     bool
	IsLatest   bool
	PoolLength int
}

type SynckerStatusInfo struct {
	Beacon     syncInfo
	S2B        syncInfo
	Shard      map[int]*syncInfo
	Crossshard map[int]*syncInfo
}

func (synckerManager *SynckerManager) GetSyncStatus(includePool bool) SynckerStatusInfo {
	info := SynckerStatusInfo{}
	info.Beacon = syncInfo{
		IsSync:   synckerManager.BeaconSyncProcess.status == RUNNING_SYNC,
		IsLatest: synckerManager.BeaconSyncProcess.isCatchUp,
	}
	info.S2B = syncInfo{
		IsSync:   synckerManager.S2BSyncProcess.status == RUNNING_SYNC,
		IsLatest: false,
	}

	info.Shard = make(map[int]*syncInfo)
	for k, v := range synckerManager.ShardSyncProcess {
		info.Shard[k] = &syncInfo{
			IsSync:   v.status == RUNNING_SYNC,
			IsLatest: v.isCatchUp,
		}
	}

	info.Crossshard = make(map[int]*syncInfo)
	for k, v := range synckerManager.CrossShardSyncProcess {
		info.Crossshard[k] = &syncInfo{
			IsSync:   v.status == RUNNING_SYNC,
			IsLatest: false,
		}
	}

	if includePool {
		info.Beacon.PoolLength = synckerManager.beaconPool.GetPoolSize()
		info.S2B.PoolLength = synckerManager.s2bPool.GetPoolSize()
		for k, _ := range synckerManager.ShardSyncProcess {
			info.Shard[k].PoolLength = synckerManager.shardPool[k].GetPoolSize()
		}
		for k, _ := range synckerManager.CrossShardSyncProcess {
			info.Crossshard[k].PoolLength = synckerManager.crossShardPool[k].GetPoolSize()
		}
	}
	return info
}

func (synckerManager *SynckerManager) IsChainReady(chainID int) bool {
	if chainID == -1 {
		return synckerManager.BeaconSyncProcess.isCatchUp
	} else if chainID >= 0 {
		return synckerManager.ShardSyncProcess[chainID].isCatchUp
	}
	return false
}

type TmpBlock struct {
	Height  uint64
	BlkHash *common.Hash
	PreHash common.Hash
	ShardID int
	Round   int
}

func (blk *TmpBlock) GetHeight() uint64 {
	return blk.Height
}

func (blk *TmpBlock) Hash() *common.Hash {
	return blk.BlkHash
}

func (blk *TmpBlock) GetPrevHash() common.Hash {
	return blk.PreHash
}

func (blk *TmpBlock) GetShardID() int {
	return blk.ShardID
}
func (blk *TmpBlock) GetRound() int {
	return 1
}

func (synckerManager *SynckerManager) GetPoolInfo(poolType byte, sID int) []common.BlockPoolInterface {
	switch poolType {
	case BeaconPoolType:
		if synckerManager.BeaconSyncProcess != nil {
			if synckerManager.BeaconSyncProcess.beaconPool != nil {
				return synckerManager.BeaconSyncProcess.beaconPool.GetPoolInfo()
			}
		}
	case ShardPoolType:
		if syncProcess, ok := synckerManager.ShardSyncProcess[sID]; ok {
			if syncProcess.shardPool != nil {
				return syncProcess.shardPool.GetPoolInfo()
			}
		}
	case S2BPoolType:
		if synckerManager.S2BSyncProcess != nil {
			if synckerManager.S2BSyncProcess.s2bPool != nil {
				return synckerManager.S2BSyncProcess.s2bPool.GetPoolInfo()
			}
		}
	case CrossShardPoolType:
		if syncProcess, ok := synckerManager.ShardSyncProcess[sID]; ok {
			if syncProcess.shardPool != nil {
				res := []common.BlockPoolInterface{}
				for fromSID, blksPool := range synckerManager.crossShardPool {
					for _, blk := range blksPool.blkPoolByHash {
						res = append(res, &TmpBlock{
							Height:  blk.GetHeight(),
							BlkHash: blk.Hash(),
							PreHash: common.Hash{},
							ShardID: fromSID,
						})
					}
				}
				return res
			}
		}
	}
	return []common.BlockPoolInterface{}
}

func (synckerManager *SynckerManager) GetPoolLatestHeight(poolType byte, bestHash string, sID int) uint64 {
	switch poolType {
	case BeaconPoolType:
		if synckerManager.BeaconSyncProcess != nil {
			if synckerManager.BeaconSyncProcess.beaconPool != nil {
				return synckerManager.BeaconSyncProcess.beaconPool.GetLatestHeight(bestHash)
			}
		}
	case ShardPoolType:
		if syncProcess, ok := synckerManager.ShardSyncProcess[sID]; ok {
			if syncProcess.shardPool != nil {
				return syncProcess.shardPool.GetLatestHeight(bestHash)
			}
		}
	case S2BPoolType:
		if synckerManager.S2BSyncProcess != nil {
			if synckerManager.S2BSyncProcess.s2bPool != nil {
				return synckerManager.S2BSyncProcess.s2bPool.GetLatestHeight(bestHash)
			}
		}
	case CrossShardPoolType:
		//TODO
		return 0
	}
	return 0
}

func (synckerManager *SynckerManager) GetAllViewByHash(poolType byte, bestHash string, sID int) []common.BlockPoolInterface {
	switch poolType {
	case BeaconPoolType:
		if synckerManager.BeaconSyncProcess != nil {
			if synckerManager.BeaconSyncProcess.beaconPool != nil {
				return synckerManager.BeaconSyncProcess.beaconPool.GetAllViewByHash(bestHash)
			}
		}
	case ShardPoolType:
		if syncProcess, ok := synckerManager.ShardSyncProcess[sID]; ok {
			if syncProcess.shardPool != nil {
				return syncProcess.shardPool.GetAllViewByHash(bestHash)
			}
		}
	default:
		//TODO
		return []common.BlockPoolInterface{}
	}
	return []common.BlockPoolInterface{}
}
