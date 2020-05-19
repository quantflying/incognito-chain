package common

import (
	"fmt"
	"math"
)

type BlockPoolInterface interface {
	GetPrevHash() Hash
	Hash() *Hash
	GetHeight() uint64
	GetShardID() int
	GetRound() int
}

type BlockInterface interface {
	GetVersion() int
	GetHeight() uint64
	Hash() *Hash
	// AddValidationField(validateData string) error
	GetProducer() string
	GetValidationField() string
	GetRound() int
	GetRoundKey() string
	GetInstructions() [][]string
	GetConsensusType() string
	GetCurrentEpoch() uint64
	GetProduceTime() int64
	GetProposeTime() int64
	GetPrevHash() Hash
	GetProposer() string
}

type ChainInterface interface {
	GetShardID() int
}

const TIMESLOT = 10

var currentRawTimeSlot = int64(0)
var currentTimeSlot = int64(0)

func CalculateTimeSlot(time int64) int64 {
	defer fmt.Println(currentTimeSlot)

	if int64(math.Floor(float64(time/TIMESLOT)))-currentRawTimeSlot > 0 {
		currentRawTimeSlot = int64(math.Floor(float64(time / TIMESLOT)))
		currentTimeSlot++
	} else {
		if int64(math.Floor(float64(time/TIMESLOT)))-currentRawTimeSlot < 0 {
			return currentTimeSlot - 1
		}
	}
	return currentTimeSlot
}
