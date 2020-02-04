package block

import "context"

type StoreBeaconDatabaseState struct {
	ctx     context.Context
	block   *BeaconBlock
	bc      BlockChain
	curView *BeaconView
	newView *BeaconView
	//app
	app []BeaconApp
}

func (s *BeaconView) NewStoreDBState(ctx context.Context) *StoreBeaconDatabaseState {
	storeDBState := &StoreBeaconDatabaseState{
		ctx:     ctx,
		bc:      s.BC,
		curView: s,
		newView: s.CloneNewView().(*BeaconView),
		app:     []BeaconApp{},
	}

	//ADD YOUR APP HERE
	storeDBState.app = append(storeDBState.app, &BeaconCoreApp{Logger: s.Logger, StoreState: storeDBState})
	storeDBState.app = append(storeDBState.app, &BeaconBridgeApp{Logger: s.Logger, StoreState: storeDBState})
	storeDBState.app = append(storeDBState.app, &BeaconPDEApp{Logger: s.Logger, StoreState: storeDBState})

	return storeDBState
}

func (s *BeaconView) StoreDatabase(ctx context.Context) error {
	state := s.NewStoreDBState(context.Background())

	for _, app := range state.app {
		err := app.storeDatabase()
		if err != nil {
			//TODO: revert db state if get error
			return err
		}
	}

	//TODO: commit
	// revert db state if commit error
	return nil
}
