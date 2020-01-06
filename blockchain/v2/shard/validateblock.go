package shard

import (
	"context"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/metadata"
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
	beaconBlocks          []BeaconBlockInterface
	crossShardBlocks      map[byte][]*CrossShardBlock
	txsToAdd              []metadata.Transaction

	isOldBeaconHeight bool
}

func (s *ShardView) ValidateBlockAndCreateNewView(ctx context.Context, block consensus.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	validateState := &ValidateBlockState{
		ctx:                   ctx,
		bc:                    s.BC,
		curView:               s,
		newView:               s.CloneNewView().(*ShardView),
		validateProposedBlock: isPreSign,
		app:                   []ShardApp{},
	}
	validateState.newView.Block = block.(*ShardBlock)
	//ADD YOUR APP HERE
	validateState.app = append(validateState.app, &CoreApp{AppData{Logger: s.Logger}})

	// Pre-Verify: check block agg signature (if already commit)
	// or we have enough data to validate this block and get beaconblocks, crossshardblock, txToAdd confirm by proposed block (not commit, = isPresign)
	for _, app := range validateState.app {
		err := app.preValidate(validateState)
		if err != nil {
			return nil, err
		}
	}
	createState := s.NewCreateState(ctx)
	if isPreSign {

		//build shardbody and check content is same
		for _, app := range createState.app {
			if err := app.buildTxFromCrossShard(createState); err != nil {
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
		//TODO: compare crossshard field
		//TODO: compare tx field
		//TODO: compare instruction field

		//build shard header
		for _, app := range createState.app {
			if err := app.buildHeader(createState); err != nil {
				return nil, err
			}
		}
		//TODO: compare header content
		for _, app := range createState.app {
			if err := app.compileBlockAndUpdateNewView(createState); err != nil {
				return nil, err
			}
		}
		//TODO: compare header content,  with newview
	} else {
		createState.body = &block.(*ShardBlock).Body
		createState.header = &block.(*ShardBlock).Header
		for _, app := range createState.app {
			if err := app.compileBlockAndUpdateNewView(createState); err != nil {
				return nil, err
			}
		}
		//TODO: compare header content,  with newview

		validateState.newView = createState.newView

	}

	return validateState.newView, nil
}
