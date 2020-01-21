package block

import (
	"context"
	"time"

	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
)

type CreateBeaconBlockState struct {
	ctx      context.Context
	bc       BlockChain
	curView  *BeaconView
	newBlock *BeaconBlock
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
	s2bBlks               map[byte][]*ShardToBeaconBlock

	rewardInstByEpoch [][]string

	shardStates                      map[byte][]ShardState
	validStakeInstructions           [][]string
	validStakePublicKeys             []string
	validStopAutoStakingInstructions [][]string
	validSwapInstructions            map[byte][][]string
	acceptedRewardInstructions       [][]string

	beaconSwapInstruction [][]string

	randomInstruction []string
	randomNumber      int64

	shardAssignInst [][]string
}

func (s *BeaconView) NewCreateState(ctx context.Context) *CreateBeaconBlockState {
	createState := &CreateBeaconBlockState{
		bc:       s.BC,
		curView:  s,
		newView:  s.CloneNewView().(*BeaconView),
		ctx:      ctx,
		app:      []BeaconApp{},
		newBlock: nil,
	}

	//ADD YOUR APP HERE
	createState.app = append(createState.app, &BeaconCoreApp{Logger: s.Logger, CreateState: createState})
	createState.app = append(createState.app, &BeaconPDEApp{Logger: s.Logger, CreateState: createState})
	return createState
}

func (s *BeaconView) CreateNewBlock(ctx context.Context, timeslot uint64, proposer string) (consensus.BlockInterface, error) {
	s.Logger.Criticalf("Creating Shard Block %+v at timeslot %v", s.GetHeight()+1, timeslot)
	createState := s.NewCreateState(ctx)
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

	createState.newBlock = &BeaconBlock{
		Body: BeaconBody{
			ShardState:   createState.shardStates,
			Instructions: instructions,
		},
	}

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

	return createState.newBlock, nil
}
