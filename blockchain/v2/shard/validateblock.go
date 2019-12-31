package shard

import (
	"context"
	"errors"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type ValidateBlockState struct {
	ctx     context.Context
	bc      BlockChain
	curView *ShardView
	//tmp
	validateProposedBlock bool
	newView               *ShardView
}

func (s *ShardView) ValidateBlock(ctx context.Context, block consensus.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	state := &ValidateBlockState{
		ctx:                   ctx,
		bc:                    s.BC,
		curView:               s,
		newView:               s.CloneNewView().(*ShardView),
		validateProposedBlock: isPreSign,
	}
	state.newView.Block = block.(*ShardBlock)

	if err := state.preValidateBlock(); err != nil {
		return nil, err
	}

	if err := state.validateBlockWithCurrentView(); err != nil {
		return nil, err
	}

	if err := state.validateBlockWithNewView(); err != nil {
		return nil, err
	}

	return state.newView, nil
}

// Pre-Verify: check block agg signature (if already commit) or we have enough data to validate this block (not commit, just propose block)
func (s *ValidateBlockState) preValidateBlock() error {
	// check valid block height
	if s.newView.GetHeight() != s.curView.GetHeight()+1 {
		return errors.New("Not valid next block")
	}

	if s.validateProposedBlock {
		// check we have enough beacon blocks from pools

		// check we have enough crossshard blocks from pools
	} else {
		//check if block has enough valid signature
	}
	return nil
}

//Verify with current view: any block data that generate in this view
func (s *ValidateBlockState) validateBlockWithCurrentView() error {
	// verify proposer in timeslot

	// verify parent hash

	// Verify timestamp

	// Verify transaction root

	// Verify ShardTx Root

	// Verify crossTransaction coin

	// Verify Action

	return nil
}

//Verify with new view: any block info that describe new view state (ie. committee root hash)
func (s *ValidateBlockState) validateBlockWithNewView() error {
	return nil
}
