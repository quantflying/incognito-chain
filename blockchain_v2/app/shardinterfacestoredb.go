package app

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
)

type StoreShardDatabaseState struct {
	ctx     context.Context
	block   blockinterface.ShardBlockInterface
	bc      *blockchainV2
	curView *ShardView
	newView *ShardView
	//app
	app []ShardApp
}

func (shardView *ShardView) NewStoreDBState(ctx context.Context) *StoreShardDatabaseState {
	storeDBState := &StoreShardDatabaseState{
		ctx:     ctx,
		bc:      shardView.bc,
		curView: shardView,
		newView: shardView.CloneNewView().(*ShardView),
		app:     []ShardApp{},
	}

	//ADD YOUR APP HERE
	storeDBState.app = append(storeDBState.app, &ShardCoreApp{Logger: shardView.Logger, storeState: storeDBState})
	storeDBState.app = append(storeDBState.app, &ShardPDEApp{Logger: shardView.Logger, StoreState: storeDBState})
	storeDBState.app = append(storeDBState.app, &ShardBridgeApp{Logger: shardView.Logger, storeState: storeDBState})

	return storeDBState
}

func (shardView *ShardView) StoreDatabase(ctx context.Context) error {
	state := shardView.NewStoreDBState(context.Background())

	for _, app := range state.app {
		err := app.storeDatabase(state)
		if err != nil {
			//TODO: revert db state if get error
			return err
		}
	}
	return nil
}
