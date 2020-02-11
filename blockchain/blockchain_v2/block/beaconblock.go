package block

import (
	"encoding/json"
	"fmt"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
)

type BeaconBlock struct {
	// ValidationData  string `json:"ValidationData"`
	ConsensusHeader ConsensusHeader
	Body            BeaconBody
	Header          BeaconHeader
}

func (beaconBlock BeaconBlock) GetBlockType() string {
	return "beacon"
}

func (beaconBlock BeaconBlock) GetBeaconHeight() uint64 {
	return beaconBlock.Header.Height
}

func (beaconBlock BeaconBlock) GetBlockProposer() string {
	return beaconBlock.ConsensusHeader.Proposer
}

func (beaconBlock BeaconBlock) GetPreviousBlockHash() common.Hash {
	return beaconBlock.Header.PreviousBlockHash
}

func (beaconBlock BeaconBlock) GetTimeslot() uint64 {
	return beaconBlock.ConsensusHeader.TimeSlot
}

func (beaconBlock BeaconBlock) GetCreateTimeslot() uint64 {
	return beaconBlock.Header.TimeSlot
}

func (beaconBlock BeaconBlock) GetBlockTimestamp() int64 {
	return beaconBlock.Header.Timestamp
}

func (beaconBlock BeaconBlock) Hash() *common.Hash {
	hash := beaconBlock.Header.Hash()
	return &hash
}

func (beaconBlock BeaconBlock) GetCurrentEpoch() uint64 {
	return beaconBlock.Header.Epoch
}

func (beaconBlock BeaconBlock) GetHeight() uint64 {
	return beaconBlock.Header.Height
}

func (beaconBlock *BeaconBlock) UnmarshalJSON(data []byte) error {
	tempBeaconBlock := &struct {
		ConsensusHeader ConsensusHeader
		Header          BeaconHeader
		Body            BeaconBody
	}{}
	err := json.Unmarshal(data, &tempBeaconBlock)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnmashallJsonShardBlockError, err)
	}
	beaconBlock.ConsensusHeader = tempBeaconBlock.ConsensusHeader
	beaconBlock.Header = tempBeaconBlock.Header
	beaconBlock.Body = tempBeaconBlock.Body
	return nil
}

func (beaconBlock *BeaconBlock) AddValidationField(validationData string) error {
	beaconBlock.ConsensusHeader.ValidationData = validationData
	return nil
}
func (beaconBlock BeaconBlock) GetValidationField() string {
	return beaconBlock.ConsensusHeader.ValidationData
}

func (beaconBlock BeaconBlock) GetRound() int {
	return beaconBlock.Header.Round
}
func (beaconBlock BeaconBlock) GetRoundKey() string {
	return fmt.Sprint(beaconBlock.Header.Height, "_", beaconBlock.Header.Round)
}

func (beaconBlock BeaconBlock) GetInstructions() [][]string {
	return beaconBlock.Body.Instructions
}

func (beaconBlock BeaconBlock) GetProducer() string {
	return beaconBlock.Header.Producer
}

func (beaconBlock BeaconBlock) GetConsensusType() string {
	return beaconBlock.Header.ConsensusType
}
