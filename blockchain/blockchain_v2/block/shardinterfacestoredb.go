package block

import "context"

type StoreDatabaseState struct {
	ctx     context.Context
	block   *ShardBlock
	bc      BlockChain
	curView *ShardView
	newView *ShardView
	//app
	app []ShardApp
}

func (s *ShardView) NewStoreDBState(ctx context.Context) *StoreDatabaseState {
	storeDBState := &StoreDatabaseState{
		ctx:     ctx,
		bc:      s.BC,
		curView: s,
		newView: s.CloneNewView().(*ShardView),
		app:     []ShardApp{},
	}

	//ADD YOUR APP HERE
	storeDBState.app = append(storeDBState.app, &ShardCoreApp{Logger: s.Logger})

	return storeDBState
}

func (s *ShardView) StoreDatabase(ctx context.Context) error {
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
