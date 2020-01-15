package block

import (
	"context"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
)

type ValidateBeaconBlockState struct {
	ctx     context.Context
	bc      BlockChain
	curView *BeaconView
	//app
	app []BeaconApp

	//tmp
	isPreSign bool
	newView   *BeaconView
}

func (s *BeaconView) NewValidateState(ctx context.Context) *ValidateBeaconBlockState {
	validateState := &ValidateBeaconBlockState{
		ctx:     ctx,
		bc:      s.BC,
		curView: s,
		newView: s.CloneNewView().(*BeaconView),
		app:     []BeaconApp{},
	}

	//ADD YOUR APP HERE
	validateState.app = append(validateState.app, &BeaconCoreApp{Logger: s.Logger, ValidateState: validateState})
	return validateState
}

func (s *BeaconView) ValidateBlockAndCreateNewView(ctx context.Context, block consensus.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	validateState := s.NewValidateState(ctx)
	validateState.newView.Block = block.(*BeaconBlock)
	validateState.isPreSign = isPreSign

	createState := s.NewCreateState(ctx)
	//block has correct basic header
	//we have enough data to validate this block and get beaconblocks, crossshardblock, txToAdd confirm by proposed block
	for _, app := range validateState.app {
		err := app.preValidate()
		if err != nil {
			return nil, err
		}
	}

	if isPreSign {
		createState.newBlock = &BeaconBlock{}
		for _, app := range createState.app {
			if err := app.buildInstructionFromShardAction(); err != nil {
				return nil, err
			}
		}

		for _, app := range createState.app {
			if err := app.buildInstructionByEpoch(); err != nil {
				return nil, err
			}
		}

		createState.newBlock = &BeaconBlock{}
		//TODO: compare body content
		for _, app := range createState.app {
			if err := app.updateNewViewFromBlock(createState.newBlock); err != nil {
				return nil, err
			}
		}

		//build shard header
		for _, app := range createState.app {
			if err := app.buildHeader(); err != nil {
				return nil, err
			}
		}

		//TODO: compare new view related content in header
		validateState.newView = createState.newView
	} else {
		//validate producer signature
		if err := (blsbftv2.BLSBFT{}.ValidateProducerSig(block)); err != nil {
			return nil, err
		}
		//validate committeee signature
		if err := (blsbftv2.BLSBFT{}.ValidateCommitteeSig(block, s.GetCommittee())); err != nil {
			s.Logger.Error("Validate fail!", s.GetCommittee())
			panic(1)
			return nil, err
		}

		createState.newView = s.CloneNewView().(*BeaconView)
		for _, app := range createState.app {
			if err := app.updateNewViewFromBlock(block.(*BeaconBlock)); err != nil {
				return nil, err
			}
		}
		validateState.newView = createState.newView
		//TODO: compare header content, with newview, necessary???
	}

	return validateState.newView, nil
}
