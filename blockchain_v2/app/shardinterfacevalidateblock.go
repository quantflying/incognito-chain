package app

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/shardblockv2"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/shardstate"

	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/consensus_v2/blsbftv2"
	"github.com/incognitochain/incognito-chain/metadata"
)

type ValidateShardBlockState struct {
	ctx     context.Context
	bc      *blockchainV2
	curView *ShardView
	//app
	app []ShardApp

	//tmp
	isPreSign        bool
	newView          *ShardView
	beaconBlocks     []blockinterface.BeaconBlockInterface
	crossShardBlocks map[byte][]shardstate.ShardState
	txsToAdd         []metadata.Transaction

	isOldBeaconHeight bool
}

func (shardView *ShardView) NewValidateState(ctx context.Context) *ValidateShardBlockState {
	validateState := &ValidateShardBlockState{
		ctx:     ctx,
		bc:      shardView.bc,
		curView: shardView,
		newView: shardView.CloneNewView().(*ShardView),
		app:     []ShardApp{},
	}

	//ADD YOUR APP HERE
	validateState.app = append(validateState.app, &ShardCoreApp{Logger: shardView.Logger, validateState: validateState})
	validateState.app = append(validateState.app, &ShardPDEApp{Logger: shardView.Logger, ValidateState: validateState})
	validateState.app = append(validateState.app, &ShardBridgeApp{Logger: shardView.Logger, validateState: validateState})
	return validateState
}

func (shardView *ShardView) ValidateBlockAndCreateNewView(ctx context.Context, block blockinterface.BlockInterface, isPreSign bool) (consensus.ChainViewInterface, error) {
	validateState := shardView.NewValidateState(ctx)
	validateState.newView.Block = block.(blockinterface.ShardBlockInterface)
	validateState.isPreSign = isPreSign

	createState := shardView.NewCreateState(ctx)
	//block has correct basic header
	//we have enough data to validate this block and get beaconblocks, crossshardblock, txToAdd confirm by proposed block
	for _, app := range validateState.app {
		err := app.preValidate()
		if err != nil {
			return nil, err
		}
	}

	if isPreSign {

		//build shardbody and check content is same
		for _, app := range createState.app {
			if err := app.buildTxFromCrossShard(); err != nil {
				return nil, err
			}
		}

		for _, app := range createState.app {
			if err := app.buildResponseTxFromTxWithMetadata(); err != nil {
				return nil, err
			}
		}

		for _, app := range createState.app {
			if err := app.processBeaconInstruction(); err != nil {
				return nil, err
			}
		}

		for _, app := range createState.app {
			if err := app.generateInstruction(); err != nil {
				return nil, err
			}
		}
		//TODO: compare crossshard field
		//TODO: compare tx field
		//TODO: compare instruction field

		createState.newBlock = &shardblockv2.ShardBlock{
			Body: block.(*shardblockv2.ShardBlock).Body,
		}

		for _, app := range createState.app {
			if err := app.updateNewViewFromBlock(block.(*shardblockv2.ShardBlock)); err != nil {
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
		if err := (blsbftv2.BLSBFT{}.ValidateCommitteeSig(block, shardView.GetCommittee())); err != nil {
			shardView.Logger.Error("Validate fail!", shardView.GetCommittee())
			panic(1)
			return nil, err
		}
		// createState.newView = shardView.CloneNewView().(*ShardView)
		// for _, app := range createState.app {
		// 	if err := app.updateNewViewFromBlock(block.(*ShardBlock)); err != nil {
		// 		return nil, err
		// 	}
		// }
		// validateState.newView = createState.newView
		//compare header content, with newview
	}

	return validateState.newView, nil
}
