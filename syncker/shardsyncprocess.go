package syncker

import (
	"context"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/wire"
	"sync"
	"time"
)

type ShardPeerState struct {
	Timestamp      int64
	BestViewHash   string
	BestViewHeight uint64
	processed      bool
}

type ShardSyncProcess struct {
	isCommittee           bool
	isCatchUp             bool
	shardID               int
	status                string                    //stop, running
	shardPeerState        map[string]ShardPeerState //peerid -> state
	shardPeerStateCh      chan *wire.MessagePeerState
	crossShardSyncProcess *CrossShardSyncProcess
	Server                Server
	Chain                 ShardChainInterface
	beaconChain           Chain
	shardPool             *BlkPool
	actionCh              chan func()
	lock                  *sync.RWMutex
}

func NewShardSyncProcess(shardID int, server Server, beaconChain BeaconChainInterface, chain ShardChainInterface) *ShardSyncProcess {
	s := &ShardSyncProcess{
		shardID:          shardID,
		status:           STOP_SYNC,
		Server:           server,
		Chain:            chain,
		beaconChain:      beaconChain,
		shardPool:        NewBlkPool("ShardPool-" + string(shardID)),
		shardPeerState:   make(map[string]ShardPeerState),
		shardPeerStateCh: make(chan *wire.MessagePeerState),

		actionCh: make(chan func()),
	}
	s.crossShardSyncProcess = NewCrossShardSyncProcess(server, s, beaconChain)

	go s.syncShardProcess()
	go s.insertShardBlockFromPool()
	return s
}

func (s *ShardSyncProcess) start() {
	if s.status == RUNNING_SYNC {
		return
	}
	s.status = RUNNING_SYNC

	go func() {
		ticker := time.NewTicker(time.Millisecond * 500)
		for {
			if s.isCommittee {
				s.crossShardSyncProcess.start()
			} else {
				s.crossShardSyncProcess.stop()
			}

			select {
			case f := <-s.actionCh:
				f()
			case shardPeerState := <-s.shardPeerStateCh:
				for sid, peerShardState := range shardPeerState.Shards {
					if int(sid) == s.shardID {
						s.shardPeerState[shardPeerState.SenderID] = ShardPeerState{
							Timestamp:      shardPeerState.Timestamp,
							BestViewHash:   peerShardState.BlockHash.String(),
							BestViewHeight: peerShardState.Height,
						}
					}
				}
			case <-ticker.C:
				s.Chain.SetReady(s.isCatchUp)
				for sender, ps := range s.shardPeerState {
					if ps.Timestamp < time.Now().Unix()-10 {
						delete(s.shardPeerState, sender)
					}
				}
			}
		}
	}()

}

func (s *ShardSyncProcess) stop() {
	s.status = STOP_SYNC
	s.crossShardSyncProcess.stop()
}

//helper function to access map atomically
func (s *ShardSyncProcess) getShardPeerStates() map[string]ShardPeerState {
	res := make(chan map[string]ShardPeerState)
	s.actionCh <- func() {
		ps := make(map[string]ShardPeerState)
		for k, v := range s.shardPeerState {
			ps[k] = v
		}
		res <- ps
	}
	return <-res
}

//periodically check pool and insert shard block to chain
func (s *ShardSyncProcess) insertShardBlockFromPool() {
	defer func() {
		if s.isCatchUp {
			time.AfterFunc(time.Millisecond*100, s.insertShardBlockFromPool)
		} else {
			time.AfterFunc(time.Second*1, s.insertShardBlockFromPool)
		}
	}()

	if !s.isCatchUp {
		return
	}
	var blk common.BlockPoolInterface
	blk = s.shardPool.GetNextBlock(s.Chain.GetBestViewHash())

	if isNil(blk) {
		return
	}

	fmt.Println("Syncker: Insert shard from pool", blk.(common.BlockInterface).GetHeight())
	s.shardPool.RemoveBlock(blk.Hash().String())
	if err := s.Chain.ValidateBlockSignatures(blk.(common.BlockInterface), s.Chain.GetCommittee()); err != nil {
		return
	}

	if err := s.Chain.InsertBlk(blk.(common.BlockInterface)); err != nil {
	}
}

func (s *ShardSyncProcess) syncShardProcess() {
	for {
		requestCnt := 0
		if s.status != RUNNING_SYNC {
			s.isCatchUp = false
			time.Sleep(time.Second * 5)
			continue
		}

		for peerID, pState := range s.getShardPeerStates() {
			requestCnt += s.streamFromPeer(peerID, pState)
		}

		if requestCnt > 0 {
			s.isCatchUp = false
			s.syncShardProcess()
		} else {
			if len(s.shardPeerState) > 0 {
				s.isCatchUp = true
			}
			time.Sleep(time.Second * 5)
		}
	}

}

func (s *ShardSyncProcess) streamFromPeer(peerID string, pState ShardPeerState) (requestCnt int) {
	if pState.processed {
		return
	}

	blockBuffer := []common.BlockInterface{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer func() {
		if requestCnt == 0 {
			pState.processed = true
		}
		cancel()
	}()

	if pState.processed {
		return
	}

	if pState.BestViewHeight <= s.Chain.GetBestViewHeight() {
		return
	}

	//fmt.Println("SYNCKER Request Shard Block", peerID, s.ShardID, s.Chain.GetBestViewHeight()+1, pState.BestViewHeight)
	ch, err := s.Server.RequestShardBlocksViaStream(ctx, peerID, s.shardID, s.Chain.GetBestViewHeight()+1, pState.BestViewHeight)
	if err != nil {
		fmt.Println("Syncker: create channel fail")
		return
	}

	requestCnt++
	insertTime := time.Now()
	for {
		select {
		case blk := <-ch:
			if !isNil(blk) {
				blockBuffer = append(blockBuffer, blk)
				if blk.(*blockchain.ShardBlock).Header.BeaconHeight > s.beaconChain.GetBestViewHeight() {
					time.Sleep(5 * time.Second)
				}
				if blk.(*blockchain.ShardBlock).Header.BeaconHeight > s.beaconChain.GetBestViewHeight() {
					return
				}
			}

			if len(blockBuffer) >= 350 || (len(blockBuffer) > 0 && (isNil(blk) || time.Since(insertTime) > time.Millisecond*1000)) {
				insertBlkCnt := 0
				for {
					time1 := time.Now()
					if successBlk, err := InsertBatchBlock(s.Chain, blockBuffer); err != nil {
						return
					} else {
						insertBlkCnt += successBlk
						fmt.Printf("Syncker Insert %d shard (from %d to %d) elaspse %f \n", successBlk, blockBuffer[0].GetHeight(), blockBuffer[len(blockBuffer)-1].GetHeight(), time.Since(time1).Seconds())
						if successBlk >= len(blockBuffer) {
							break
						}
						blockBuffer = blockBuffer[successBlk:]
					}
				}

				insertTime = time.Now()
				blockBuffer = []common.BlockInterface{}
			}

			if isNil(blk) && len(blockBuffer) == 0 {
				return
			}
		}
	}

}
