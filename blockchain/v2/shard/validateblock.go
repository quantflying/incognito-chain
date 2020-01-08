package shard

import (
	"context"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/metadata"
)

type ValidateBlockState struct {
	ctx     context.Context
	bc      BlockChain
	curView *ShardView
	//app
	app []ShardApp

	//tmp
	isPreSign        bool
	newView          *ShardView
	beaconBlocks     []BeaconBlockInterface
	crossShardBlocks map[byte][]*CrossShardBlock
	txsToAdd         []metadata.Transaction

	isOldBeaconHeight bool
}

func (s *ShardView) NewValidateState(ctx context.Context) *ValidateBlockState {
	validateState := &ValidateBlockState{
		ctx:     ctx,
		bc:      s.BC,
		curView: s,
		newView: s.CloneNewView().(*ShardView),
		app:     []ShardApp{},
	}

	//ADD YOUR APP HERE
	validateState.app = append(validateState.app, &CoreApp{AppData{Logger: s.Logger}})
	return validateState
}

func (s *ShardView) ValidateBlockAndCreateNewView(ctx context.Context, block consensus.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	validateState := s.NewValidateState(ctx)
	validateState.newView.Block = block.(*ShardBlock)
	validateState.isPreSign = isPreSign

	createState := s.NewCreateState(ctx)
	//block has correct basic header
	//we have enough data to validate this block and get beaconblocks, crossshardblock, txToAdd confirm by proposed block
	for _, app := range validateState.app {
		err := app.preValidate(validateState)
		if err != nil {
			return nil, err
		}
	}

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
		//TODO: compare new view related content in header

	} else {
		//validate producer signature
		if err := (blsbftv2.BLSBFT{}.ValidateProducerSig(block)); err != nil {
			return nil, err
		}
		//validate committeee signature
		if err := (blsbftv2.BLSBFT{}.ValidateCommitteeSig(block, s.GetCommittee())); err != nil {
			return nil, err
		}

		//TODO: check block ehader content => not necessary at this time
		//createState.newBlock.Body = block.(*ShardBlock).Body
		//createState.newBlock.Header = block.(*ShardBlock).Header
		//for _, app := range createState.app {
		//	if err := app.createNewViewFromBlock(createState); err != nil {
		//		return nil, err
		//	}
		//}
		//compare header content, with newview
	}

	validateState.newView = createState.newView
	return validateState.newView, nil
}
