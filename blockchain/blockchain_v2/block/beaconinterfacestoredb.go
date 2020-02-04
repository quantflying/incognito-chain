package block

import "context"

type StoreBeaconDatabaseState struct {
	ctx     context.Context
	block   *ShardBlock
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
	storeDBState.app = append(storeDBState.app, &BeaconCoreApp{Logger: s.Logger})

	return storeDBState
}

func (s *BeaconView) StoreDatabase(block *ShardBlock, newView *BeaconView) error {
	state := s.NewStoreDBState(context.Background())

	for _, app := range state.app {
		err := app.storeDatabase(state)
		if err != nil {
			//TODO: revert db state if get error
			return err
		}
	}

	//TODO: commit
	// revert db state if commit error
	return nil
}
