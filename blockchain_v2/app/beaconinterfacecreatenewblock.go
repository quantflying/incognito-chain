package app

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/beaconblockv2"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/shardstate"
	"time"
)

type CreateBeaconBlockState struct {
	ctx      context.Context
	bc       *blockchainV2
	curView  *BeaconView
	newBlock blockinterface.BeaconBlockInterface
	newView  *BeaconView

	//app
	app []BeaconApp

	//tmp
	createTimeStamp int64
	createTimeSlot  uint64
	proposer        string

	isNewEpoch            bool
	isEndEpoch            bool
	isGettingRandomNumber bool
	isRandomTime          bool
	s2bBlks               map[byte][]blockinterface.ShardToBeaconBlockInterface

	rewardInstByEpoch [][]string

	shardStates                      map[byte][]shardstate.ShardState
	validStakeInstructions           [][]string
	validStakePublicKeys             []string
	validStopAutoStakingInstructions [][]string
	validSwapInstructions            map[byte][][]string
	acceptedRewardInstructions       [][]string

	beaconSwapInstruction [][]string

	randomInstruction []string
	randomNumber      int64

	shardAssignInst [][]string

	bridgeInstructions   [][]string
	statefulInstructions [][]string
}

func (beaconView *BeaconView) NewCreateState(ctx context.Context) *CreateBeaconBlockState {
	createState := &CreateBeaconBlockState{
		bc:       beaconView.bc,
		curView:  beaconView,
		newView:  beaconView.CloneNewView().(*BeaconView),
		ctx:      ctx,
		app:      []BeaconApp{},
		newBlock: nil,
	}

	//ADD YOUR APP HERE
	createState.app = append(createState.app, &BeaconCoreApp{Logger: beaconView.Logger, CreateState: createState, bc: beaconView.bc})
	createState.app = append(createState.app, &BeaconPDEApp{Logger: beaconView.Logger, CreateState: createState})
	createState.app = append(createState.app, &BeaconBridgeApp{Logger: beaconView.Logger, createState: createState})
	return createState
}

func (beaconView *BeaconView) CreateNewBlock(ctx context.Context, timeslot uint64, proposer string) (blockinterface.BlockInterface, error) {
	beaconView.Logger.Criticalf("Creating Beacon Block %+v at timeslot %v", beaconView.GetHeight()+1, timeslot)
	createState := beaconView.NewCreateState(ctx)
	createState.createTimeStamp = time.Now().Unix()
	createState.createTimeSlot = timeslot
	createState.proposer = proposer
	// createState.newBlock = &BeaconBlock{}
	//pre processing
	for _, app := range createState.app {
		if err := app.preCreateBlock(); err != nil {
			return nil, err
		}
	}

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

	instructions := [][]string{}
	// instructions = append(instructions, createState.randomInstruction)
	instructions = append(instructions, createState.rewardInstByEpoch...)
	instructions = append(instructions, createState.validStakeInstructions...)
	instructions = append(instructions, createState.validStopAutoStakingInstructions...)
	instructions = append(instructions, createState.acceptedRewardInstructions...)
	instructions = append(instructions, createState.beaconSwapInstruction...)
	instructions = append(instructions, createState.shardAssignInst...)

	instructions = append(instructions, createState.bridgeInstructions...)
	instructions = append(instructions, createState.statefulInstructions...)

	createState.newBlock = &beaconblockv2.BeaconBlock{
		Body: beaconblockv2.BeaconBody{
			ShardState:   createState.shardStates,
			Instructions: instructions,
		},
	}

	for _, app := range createState.app {
		if err := app.updateNewViewFromBlock(createState.newBlock); err != nil {
			return nil, err
		}
	}

	//build header
	for _, app := range createState.app {
		if err := app.buildHeader(); err != nil {
			return nil, err
		}
	}
	return createState.newBlock, nil
}
