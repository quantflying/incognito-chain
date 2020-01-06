package shard

import (
	"context"
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
	newView  *ShardView

	//tmp
	newConfirmBeaconHeight uint64
	beaconBlocks           []common.BlockInterface
	crossShardBlocks       map[byte][]*CrossShardBlock

	//app
	app []ShardApp

	txToRemove               []metadata.Transaction
	txsToAdd                 []metadata.Transaction
	txsFromMetadataTx        []metadata.Transaction
	txsFromBeaconInstruction []metadata.Transaction
	errInstruction           [][]string
	stakingTx                map[string]string
	newShardPendingValidator []string

	//body data
	body *ShardBody
	//header
	header *ShardHeader
}

func (s *ShardView) NewCreateState(ctx context.Context) *CreateNewBlockState {
	createState := &CreateNewBlockState{
		bc:       s.BC,
		curView:  s,
		newView:  s.CloneNewView().(*ShardView),
		ctx:      ctx,
		app:      []ShardApp{},
		newBlock: nil,
	}

	//ADD YOUR APP HERE
	createState.app = append(createState.app, &CoreApp{AppData{Logger: s.Logger}})
	return createState
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
	createState := s.NewCreateState(ctx)

	//pre processing
	for _, app := range createState.app {
		if err := app.preCreateBlock(createState); err != nil {
			return nil, err
		}
	}

	//build shardbody
	for _, app := range createState.app {
		if err := app.buildTxFromCrossShard(createState); err != nil {
			return nil, err
		}
	}

	for _, app := range createState.app {
		if err := app.buildTxFromMemPool(createState); err != nil {
			return nil, err
		}
	}

	for _, app := range createState.app {
		if err := app.buildResponseTxFromTxWithMetadata(createState); err != nil {
			return nil, err
		}
	}

	for _, app := range createState.app {
		if err := app.processBeaconInstruction(createState); err != nil {
			return nil, err
		}
	}

	for _, app := range createState.app {
		if err := app.generateInstruction(createState); err != nil {
			return nil, err
		}
	}

	//build shard header
	for _, app := range createState.app {
		if err := app.buildHeader(createState); err != nil {
			return nil, err
		}
	}

	//post processing
	for _, app := range createState.app {
		if err := app.compileBlockAndUpdateNewView(createState); err != nil {
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
