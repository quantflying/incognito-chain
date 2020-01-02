package shard

import (
	"context"
	v2 "github.com/incognitochain/incognito-chain/blockchain/v2"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type ValidateBlockState struct {
	ctx     context.Context
	bc      BlockChain
	curView *ShardView
	//app
	app []ShardApp

	//tmp
	validateProposedBlock bool
	newView               *ShardView
	beaconBlocks          []v2.BeaconBlockInterface
	isOldBeaconHeight     bool
}

func (s *ShardView) ValidateBlock(ctx context.Context, block consensus.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	state := &ValidateBlockState{
		ctx:                   ctx,
		bc:                    s.BC,
		curView:               s,
		newView:               s.CreateNewViewFromBlock(block).(*ShardView),
		validateProposedBlock: isPreSign,

		app: []ShardApp{},
	}
	//ADD YOUR APP HERE
	state.app = append(state.app, &CoreApp{AppData{Logger: s.Logger}})

	// Pre-Verify: check block agg signature (if already commit) or we have enough data to validate this block (not commit, just propose block)
	for _, app := range state.app {
		err := app.preValidate(state)
		if err != nil {
			//TODO: revert db state if get error
			return nil, err
		}
	}

	//Verify with current view: any block data that generate in this view
	for _, app := range state.app {
		err := app.validateWithCurrentView(state)
		if err != nil {
			//TODO: revert db state if get error
			return nil, err
		}
	}

	//Verify with new view: any block info that describe new view state (ie. committee root hash)
	for _, app := range state.app {
		err := app.validateWithNewView(state)
		if err != nil {
			//TODO: revert db state if get error
			return nil, err
		}
	}

	return state.newView, nil
}
