package shard

import (
	"context"
	"errors"
	v2 "github.com/incognitochain/incognito-chain/blockchain/v2"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type ValidateBlockState struct {
	ctx     context.Context
	bc      BlockChain
	curView *ShardView
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
	}

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
	shardID := s.curView.ShardID
	// check valid block height
	if s.newView.GetHeight() != s.curView.GetHeight()+1 {
		return errors.New("Not valid next block")
	}

	if s.validateProposedBlock {
		// check we have enough beacon blocks from pools
		newBlock := s.newView.GetBlock().(*ShardBlock)
		oldBlock := s.curView.GetBlock().(*ShardBlock)

		if newBlock.Header.BeaconHeight < oldBlock.Header.BeaconHeight {
			return errors.New("Beaconheight is not valid")
		}
		if newBlock.Header.BeaconHeight > oldBlock.Header.BeaconHeight {
			s.isOldBeaconHeight = true
		}
		if newBlock.Header.BeaconHeight > oldBlock.Header.BeaconHeight {
			s.isOldBeaconHeight = false
			beaconBlocks := s.bc.GetValidBeaconBlockFromPool()
			if len(beaconBlocks) == int(newBlock.Header.BeaconHeight-oldBlock.Header.BeaconHeight) {
				s.beaconBlocks = beaconBlocks
			} else {
				return errors.New("Not enough beacon to validate")
			}
		}

		// check we have enough crossshard blocks from pools
		allConfirmCrossShard := make(map[byte][]interface{})
		for _, beaconBlk := range s.beaconBlocks {
			confirmCrossShard := beaconBlk.GetConfirmedCrossShardBlockToShard()
			for fromShardID, v := range confirmCrossShard[shardID] {
				for _, crossShard := range v {
					allConfirmCrossShard[fromShardID] = append(allConfirmCrossShard[fromShardID], crossShard)
				}
			}
		}
		//TODO: get crossshard from pool and check if we have enough to validate
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
