package shard

type StoreDatabaseState struct {
	block   *ShardBlock
	bc      BlockChain
	curView *ShardView
	newView *ShardView
	//app
	app []App
}

func (s *ShardView) StoreDatabase(block *ShardBlock, newView *ShardView) error {
	state := &StoreDatabaseState{
		bc:      s.BC,
		curView: s,
		newView: newView,
		app:     []App{},
	}
	//ADD YOUR APP HERE
	state.app = append(state.app, &CoreApp{AppData{Logger: s.Logger}})

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
