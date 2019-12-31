package shard

import (
	"context"
	"github.com/incognitochain/incognito-chain/blockchain"
	v2 "github.com/incognitochain/incognito-chain/blockchain/v2"
	consensus "github.com/incognitochain/incognito-chain/consensus_v2"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"math/rand"
	"time"
)

type CreateNewBlockState struct {
	ctx          context.Context
	bc           v2.BlockChain
	oldShardView *ShardView

	//tmp
	newShardView           *ShardView
	newConfirmBeaconHeight uint64
	//body
	instructions     [][]string
	crossTransaction map[byte][]blockchain.CrossTransaction
	transactions     []metadata.Transaction
	//header

}

func (s *ShardView) CreateNewBlock(ctx context.Context, timeslot uint64, proposer string) (consensus.BlockInterface, error) {
	s.Logger.Criticalf("Creating Shard Block %+v at timeslot %v", s.GetHeight()+1, timeslot)
	state := &CreateNewBlockState{
		bc:           s.BC,
		oldShardView: s,
		newShardView: s.CloneNewView().(*ShardView),
		ctx:          ctx,
	}

	if err := state.preProcessForCreatingNewShardBlock(); err != nil {
		return nil, err
	}

	if err := state.buildingShardBody(); err != nil {
		return nil, err
	}

	if err := state.buildingShardHeader(); err != nil {
		return nil, err
	}

	if err := state.postProcessForCreatingNewShardBlock(); err != nil {
		return nil, err
	}

	block := &ShardBlock{
		Body: ShardBody{
			Transactions:      state.transactions,
			Instructions:      state.instructions,
			CrossTransactions: state.crossTransaction,
		},
		Header: ShardHeader{
			Timestamp:         time.Now().Unix(),
			Version:           1,
			BeaconHeight:      1,
			Epoch:             1,
			Round:             1,
			Height:            s.Block.GetHeight() + 1,
			PreviousBlockHash: *s.Block.Hash(),
		},
		ConsensusHeader: ConsensusHeader{
			TimeSlot: timeslot,
			Proposer: proposer,
		},
	}
	return block, nil
}

func (s *ShardView) CreateNewBlockAndView(ctx context.Context, timeslot uint64, proposer string) (consensus.BlockInterface, error) {
	s.Logger.Criticalf("Creating Shard Block %+v at timeslot %v", s.GetHeight()+1, timeslot)
	state := &CreateNewBlockState{
		bc:           s.BC,
		oldShardView: s,
		newShardView: s.CloneNewView().(*ShardView),
		ctx:          ctx,
	}

	if err := state.preProcessForCreatingNewShardBlock(); err != nil {
		return nil, err
	}

	if err := state.buildingShardBody(); err != nil {
		return nil, err
	}

	if err := state.buildingShardHeader(); err != nil {
		return nil, err
	}

	if err := state.postProcessForCreatingNewShardBlock(); err != nil {
		return nil, err
	}

	block := &ShardBlock{
		Body: ShardBody{
			Transactions:      state.transactions,
			Instructions:      state.instructions,
			CrossTransactions: state.crossTransaction,
		},
		Header: ShardHeader{
			Timestamp:         time.Now().Unix(),
			Version:           1,
			BeaconHeight:      1,
			Epoch:             1,
			Round:             1,
			Height:            s.Block.GetHeight() + 1,
			PreviousBlockHash: *s.Block.Hash(),
		},
		ConsensusHeader: ConsensusHeader{
			TimeSlot: timeslot,
			Proposer: proposer,
		},
	}
	return block, nil
}

/*
	Preprocessing for create new shard block:
	- Get number of beacon blocks to confirm
*/
func (state *CreateNewBlockState) preProcessForCreatingNewShardBlock() error {

	bc := state.bc
	beaconHeight, err := state.bc.GetCurrentBeaconHeight()
	if err != nil {
		return err
	}
	oldShardView := state.oldShardView

	// limit number of beacon block confirmed in a shard block
	if beaconHeight-oldShardView.GetHeight() > blockchain.MAX_BEACON_BLOCK {
		beaconHeight = oldShardView.GetHeight() + blockchain.MAX_BEACON_BLOCK
	}

	// we only confirm shard blocks within an epoch
	epoch := beaconHeight / bc.GetChainParams().Epoch

	if epoch > oldShardView.GetEpoch() { // if current epoch > oldShardView 1 epoch, we only confirm beacon block within an epoch
		beaconHeight = oldShardView.GetEpoch() * bc.GetChainParams().Epoch
		epoch = oldShardView.GetEpoch() + 1
	}
	state.newConfirmBeaconHeight = beaconHeight
	return nil
}

func (state *CreateNewBlockState) buildingShardBody() error {
	shardID := state.oldShardView.ShardID
	tempPrivateKey := createTempKeyset()

	//TODO: get cross shard data from crossshard block and build crossshard tx from these data
	state.crossTransaction = state.getCrossShardData(shardID)
	if err := state.buildCrossShardTx(&tempPrivateKey, shardID); err != nil {
		return err
	}

	//TODO: build tx from mempool
	if err := state.buildTxFromMemPool(&tempPrivateKey); err != nil {
		return err
	}

	//TODO: build reponse tx from metadata tx
	if err := state.buildResponseTxFromTxWithMetadata(&tempPrivateKey); err != nil {
		return err
	}

	//TODO: process instruction from
	if err := state.processInstructionFromBeacon(); err != nil {
		return err
	}

	//TODO: build instruction to beacon
	if err := state.generateInstruction(); err != nil {
		return err
	}
	return nil
}

func (state *CreateNewBlockState) buildingShardHeader() error {
	return nil
}

func (state *CreateNewBlockState) postProcessForCreatingNewShardBlock() error {
	return nil
}

func (state *CreateNewBlockState) getCrossShardData(toShard byte) map[byte][]blockchain.CrossTransaction {
	crossTransactions := make(map[byte][]blockchain.CrossTransaction)
	return crossTransactions
}

func (state *CreateNewBlockState) buildCrossShardTx(privatekey *privacy.PrivateKey, shardID byte) error {
	return nil
}

func (state *CreateNewBlockState) buildTxFromMemPool(privatekey *privacy.PrivateKey) error {
	return nil
}

func (state *CreateNewBlockState) buildResponseTxFromTxWithMetadata(privatekey *privacy.PrivateKey) error {
	return nil
}

func (state *CreateNewBlockState) processInstructionFromBeacon() error {
	return nil
}

func (state *CreateNewBlockState) generateInstruction() error {
	return nil
}

func createTempKeyset() privacy.PrivateKey {
	rand.Seed(time.Now().UnixNano())
	seed := make([]byte, 16)
	rand.Read(seed)
	return privacy.GeneratePrivateKey(seed)
}
