package shard

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"math/rand"
	"time"
)

type CreateNewBlockState struct {
	ctx      context.Context
	bc       BlockChain
	curView  *ShardView
	newBlock *ShardBlock
	//tmp
	newView                *ShardView
	newConfirmBeaconHeight uint64
	newBeaconBlocks        []common.BlockInterface

	//app
	app                      []ShardApp
	txToRemove               []metadata.Transaction
	txsToAdd                 []metadata.Transaction
	txsFromMetadataTx        []metadata.Transaction
	txsFromBeaconInstruction []metadata.Transaction
	errInstruction           [][]string
	stakingTx                map[string]string
	newShardPendingValidator []string

	//body
	instructions     [][]string
	crossTransaction map[byte][]blockchain.CrossTransaction
	transactions     []metadata.Transaction
	//header

}

func (s *ShardView) CreateNewBlock(ctx context.Context, timeslot uint64, proposer string) (consensus.BlockInterface, error) {
	s.Logger.Criticalf("Creating Shard Block %+v at timeslot %v", s.GetHeight()+1, timeslot)
	block := &ShardBlock{
		Body: ShardBody{},
		Header: ShardHeader{
			Timestamp:         time.Now().Unix(),
			Version:           1,
			BeaconHeight:      1,
			Epoch:             1,
			Round:             1,
			Height:            s.Block.GetHeight() + 1,
			PreviousBlockHash: *s.Block.Hash(),
		},
		ConsensusHeader: ConsensusHeader{
			TimeSlot: timeslot,
			Proposer: proposer,
		},
	}
	state := &CreateNewBlockState{
		bc:       s.BC,
		curView:  s,
		newView:  s.CloneNewView().(*ShardView),
		ctx:      ctx,
		app:      []ShardApp{},
		newBlock: block,
	}
	//ADD YOUR APP HERE
	state.app = append(state.app, &CoreApp{AppData{Logger: s.Logger}})

	//pre processing
	for _, app := range state.app {
		if err := app.preProcess(state); err != nil {
			return nil, err
		}
	}

	//build shardbody
	for _, app := range state.app {
		if err := app.buildTxFromCrossShard(state); err != nil {
			return nil, err
		}
	}

	for _, app := range state.app {
		if err := app.buildTxFromMemPool(state); err != nil {
			return nil, err
		}
	}

	for _, app := range state.app {
		if err := app.buildResponseTxFromTxWithMetadata(state); err != nil {
			return nil, err
		}
	}

	for _, app := range state.app {
		if err := app.processBeaconInstruction(state); err != nil {
			return nil, err
		}
	}

	for _, app := range state.app {
		if err := app.generateInstruction(state); err != nil {
			return nil, err
		}
	}

	//build shard header
	for _, app := range state.app {
		if err := app.buildHeader(state); err != nil {
			return nil, err
		}
	}

	//post processing
	for _, app := range state.app {
		if err := app.postProcessAndCompile(state); err != nil {
			return nil, err
		}
	}

	return block, nil
}

func createTempKeyset() privacy.PrivateKey {
	rand.Seed(time.Now().UnixNano())
	seed := make([]byte, 16)
	rand.Read(seed)
	return privacy.GeneratePrivateKey(seed)
}
