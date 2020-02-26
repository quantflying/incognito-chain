package app

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
)

type StoreBeaconDatabaseState struct {
	ctx     context.Context
	block   blockinterface.BeaconBlockInterface
	bc      *blockchainV2
	curView *BeaconView
	newView *BeaconView
	//app
	app []BeaconApp
}

func (beaconView *BeaconView) NewStoreDBState(ctx context.Context) *StoreBeaconDatabaseState {
	storeDBState := &StoreBeaconDatabaseState{
		ctx:     ctx,
		bc:      beaconView.bc,
		block:   beaconView.Block,
		curView: beaconView,
		newView: beaconView.CloneNewView().(*BeaconView),
		app:     []BeaconApp{},
	}

	//ADD YOUR APP HERE
	storeDBState.app = append(storeDBState.app, &BeaconCoreApp{Logger: beaconView.Logger, StoreState: storeDBState})
	storeDBState.app = append(storeDBState.app, &BeaconBridgeApp{Logger: beaconView.Logger, storeState: storeDBState})
	storeDBState.app = append(storeDBState.app, &BeaconPDEApp{Logger: beaconView.Logger, StoreState: storeDBState})

	return storeDBState
}

func (beaconView *BeaconView) StoreDatabase(ctx context.Context) error {
	state := beaconView.NewStoreDBState(context.Background())

	for _, app := range state.app {
		err := app.storeDatabase()
		if err != nil {
			//TODO: revert db state if get error
			return err
		}
	}
	return nil
}
