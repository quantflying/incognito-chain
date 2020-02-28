package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain_v2/btc"
	"github.com/incognitochain/incognito-chain/blockchain_v2/params"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/shardstate"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
)

func buildInstructionFromBlock(s2bBlk blockinterface.ShardToBeaconBlockInterface, curView *BeaconView) ([][]string, []string, map[byte][][]string, []string, [][]string) {
	header := s2bBlk.GetShardHeader()
	shardID := header.GetShardID()
	instructions := s2bBlk.GetInstructions()
	shardStates := make(map[byte]shardstate.ShardState)
	stakeInstructions := [][]string{}
	swapInstructions := make(map[byte][][]string)
	stopAutoStakingInstructions := [][]string{}
	stopAutoStakingInstructionsFromBlock := [][]string{}
	stakeInstructionFromShardBlock := [][]string{}
	swapInstructionFromShardBlock := [][]string{}
	stakeBeaconPublicKeys := []string{}
	stakeShardPublicKeys := []string{}
	stakeBeaconTx := []string{}
	stakeShardTx := []string{}
	stakeShardRewardReceiver := []string{}
	stakeBeaconRewardReceiver := []string{}
	stakeShardAutoStaking := []string{}
	stakeBeaconAutoStaking := []string{}
	stopAutoStakingPublicKeys := []string{}
	tempValidStakePublicKeys := []string{}
	acceptedBlockRewardInfo := metadata.NewAcceptedBlockRewardInfo(shardID, header.GetTotalTxsFee(), header.GetHeight())
	acceptedRewardInstructions, err := acceptedBlockRewardInfo.GetStringFormat()
	if err != nil {
		// if err then ignore accepted reward instruction
		acceptedRewardInstructions = []string{}
	}

	shardState := shardstate.ShardState{}
	shardState.CrossShard = make([]byte, len(header.GetCrossShardBitMap()))
	copy(shardState.CrossShard, header.GetCrossShardBitMap())
	shardState.Hash = *header.GetHash()
	shardState.Height = header.GetHeight()
	shardStates[shardID] = shardState

	for _, instruction := range instructions {
		if len(instruction) > 0 {
			if instruction[0] == StakeAction {
				stakeInstructionFromShardBlock = append(stakeInstructionFromShardBlock, instruction)
			}
			if instruction[0] == SwapAction {
				//- ["swap" "inPubkey1,inPubkey2,..." "outPupkey1, outPubkey2,..." "shard" "shardID"]
				//- ["swap" "inPubkey1,inPubkey2,..." "outPupkey1, outPubkey2,..." "beacon"]
				// validate swap instruction
				// only allow shard to swap committee for it self
				if instruction[3] == "beacon" {
					continue
				}
				if instruction[3] == "shard" && len(instruction) != 6 && instruction[4] != strconv.Itoa(int(shardID)) {
					continue
				}
				swapInstructions[shardID] = append(swapInstructions[shardID], instruction)
			}
			if instruction[0] == StopAutoStake {
				if len(instruction) != 2 {
					continue
				}
				stopAutoStakingInstructionsFromBlock = append(stopAutoStakingInstructionsFromBlock, instruction)
			}
		}
	}

	if len(stakeInstructionFromShardBlock) != 0 {
		Logger.log.Info("Beacon Producer/ Process Stakers List ", stakeInstructionFromShardBlock)
	}
	if len(swapInstructions[shardID]) != 0 {
		Logger.log.Info("Beacon Producer/ Process Stakers List ", swapInstructionFromShardBlock)
	}

	// Process Stake Instruction form Shard Block
	// Validate stake instruction => extract only valid stake instruction
	for _, stakeInstruction := range stakeInstructionFromShardBlock {
		if len(stakeInstruction) != 6 {
			continue
		}
		var tempStakePublicKey []string
		newBeaconCandidate, newShardCandidate := getStakeValidatorArrayString(stakeInstruction)
		assignShard := true
		if !reflect.DeepEqual(newBeaconCandidate, []string{}) {
			tempStakePublicKey = make([]string, len(newBeaconCandidate))
			copy(tempStakePublicKey, newBeaconCandidate[:])
			assignShard = false
		} else {
			tempStakePublicKey = make([]string, len(newShardCandidate))
			copy(tempStakePublicKey, newShardCandidate[:])
		}
		// list of stake public keys and stake transaction and reward receiver must have equal length
		if len(tempStakePublicKey) != len(strings.Split(stakeInstruction[3], ",")) && len(strings.Split(stakeInstruction[3], ",")) != len(strings.Split(stakeInstruction[4], ",")) && len(strings.Split(stakeInstruction[4], ",")) != len(strings.Split(stakeInstruction[5], ",")) {
			continue
		}
		tempStakePublicKey = curView.GetValidStakers(tempStakePublicKey)
		tempStakePublicKey = common.GetValidStaker(stakeShardPublicKeys, tempStakePublicKey)
		tempStakePublicKey = common.GetValidStaker(stakeBeaconPublicKeys, tempStakePublicKey)
		if len(tempStakePublicKey) > 0 {
			if assignShard {
				stakeShardPublicKeys = append(stakeShardPublicKeys, tempStakePublicKey...)
				for i, v := range strings.Split(stakeInstruction[1], ",") {
					if common.IndexOfStr(v, tempStakePublicKey) > -1 {
						stakeShardTx = append(stakeShardTx, strings.Split(stakeInstruction[3], ",")[i])
						stakeShardRewardReceiver = append(stakeShardRewardReceiver, strings.Split(stakeInstruction[4], ",")[i])
						stakeShardAutoStaking = append(stakeShardAutoStaking, strings.Split(stakeInstruction[5], ",")[i])
					}
				}
			} else {
				stakeBeaconPublicKeys = append(stakeBeaconPublicKeys, tempStakePublicKey...)
				for i, v := range strings.Split(stakeInstruction[1], ",") {
					if common.IndexOfStr(v, tempStakePublicKey) > -1 {
						stakeBeaconTx = append(stakeBeaconTx, strings.Split(stakeInstruction[3], ",")[i])
						stakeBeaconRewardReceiver = append(stakeBeaconRewardReceiver, strings.Split(stakeInstruction[4], ",")[i])
						stakeBeaconAutoStaking = append(stakeBeaconAutoStaking, strings.Split(stakeInstruction[5], ",")[i])
					}
				}
			}
		}
	}
	if len(stakeShardPublicKeys) > 0 {
		tempValidStakePublicKeys = append(tempValidStakePublicKeys, stakeShardPublicKeys...)
		stakeInstructions = append(stakeInstructions, []string{StakeAction, strings.Join(stakeShardPublicKeys, ","), "shard", strings.Join(stakeShardTx, ","), strings.Join(stakeShardRewardReceiver, ","), strings.Join(stakeShardAutoStaking, ",")})
	}
	if len(stakeBeaconPublicKeys) > 0 {
		tempValidStakePublicKeys = append(tempValidStakePublicKeys, stakeBeaconPublicKeys...)
		stakeInstructions = append(stakeInstructions, []string{StakeAction, strings.Join(stakeBeaconPublicKeys, ","), "beacon", strings.Join(stakeBeaconTx, ","), strings.Join(stakeBeaconRewardReceiver, ","), strings.Join(stakeBeaconAutoStaking, ",")})
	}
	for _, instruction := range stopAutoStakingInstructionsFromBlock {
		allCommitteeValidatorCandidate := []string{}
		// avoid dead lock
		// if producer new block then lock beststate
		allCommitteeValidatorCandidate = curView.getAllCommitteeValidatorCandidateFlattenList()
		tempStopAutoStakingPublicKeys := strings.Split(instruction[1], ",")
		for _, tempStopAutoStakingPublicKey := range tempStopAutoStakingPublicKeys {
			if common.IndexOfStr(tempStopAutoStakingPublicKey, allCommitteeValidatorCandidate) > -1 {
				stopAutoStakingPublicKeys = append(stopAutoStakingPublicKeys, tempStopAutoStakingPublicKey)
			}
		}
	}
	if len(stopAutoStakingPublicKeys) > 0 {
		stopAutoStakingInstructions = append(stopAutoStakingInstructions, []string{StopAutoStake, strings.Join(stopAutoStakingPublicKeys, ",")})
	}

	return stakeInstructions, tempValidStakePublicKeys, swapInstructions, acceptedRewardInstructions, stopAutoStakingInstructions
}

func (beaconView *BeaconView) getAllCommitteeValidatorCandidateFlattenList() []string {
	res := []string{}
	for _, committee := range beaconView.ShardCommittee {
		committeeStr, err := incognitokey.CommitteeKeyListToString(committee)
		if err != nil {
			panic(err)
		}
		res = append(res, committeeStr...)
	}
	for _, pendingValidator := range beaconView.ShardPendingValidator {
		pendingValidatorStr, err := incognitokey.CommitteeKeyListToString(pendingValidator)
		if err != nil {
			panic(err)
		}
		res = append(res, pendingValidatorStr...)
	}

	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconCommittee)
	if err != nil {
		panic(err)
	}
	res = append(res, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconPendingValidator)
	if err != nil {
		panic(err)
	}
	res = append(res, beaconPendingValidatorStr...)

	candidateBeaconWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateBeaconWaitingForCurrentRandomStr...)

	candidateBeaconWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateBeaconWaitingForNextRandomStr...)

	candidateShardWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateShardWaitingForCurrentRandomStr...)

	candidateShardWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateShardWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	res = append(res, candidateShardWaitingForNextRandomStr...)
	return res
}

func (beaconView *BeaconView) GetValidStakers(stakers []string) []string {
	beaconView.Lock.RLock()
	defer beaconView.Lock.RUnlock()

	for _, committees := range beaconView.ShardCommittee {
		committeesStr, err := incognitokey.CommitteeKeyListToString(committees)
		if err != nil {
			panic(err)
		}
		stakers = common.GetValidStaker(committeesStr, stakers)
	}
	for _, validators := range beaconView.ShardPendingValidator {
		validatorsStr, err := incognitokey.CommitteeKeyListToString(validators)
		if err != nil {
			panic(err)
		}
		stakers = common.GetValidStaker(validatorsStr, stakers)
	}

	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconCommittee)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(beaconCommitteeStr, stakers)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconPendingValidator)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(beaconPendingValidatorStr, stakers)

	candidateBeaconWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateBeaconWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(candidateBeaconWaitingForCurrentRandomStr, stakers)

	candidateBeaconWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateBeaconWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(candidateBeaconWaitingForNextRandomStr, stakers)

	candidateShardWaitingForCurrentRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(candidateShardWaitingForCurrentRandomStr, stakers)

	candidateShardWaitingForNextRandomStr, err := incognitokey.CommitteeKeyListToString(beaconView.CandidateShardWaitingForNextRandom)
	if err != nil {
		panic(err)
	}
	stakers = common.GetValidStaker(candidateShardWaitingForNextRandomStr, stakers)
	return stakers
}

func (beaconView *BeaconView) buildInstRewardForBeacons(epoch uint64, totalReward map[common.Hash]uint64) ([][]string, error) {
	resInst := [][]string{}
	baseRewards := map[common.Hash]uint64{}
	for key, value := range totalReward {
		baseRewards[key] = value / uint64(len(beaconView.BeaconCommittee))
	}
	for _, beaconpublickey := range beaconView.BeaconCommittee {
		// indicate reward pubkey
		singleInst, err := metadata.BuildInstForBeaconReward(baseRewards, beaconpublickey.GetNormalKey())
		if err != nil {
			Logger.log.Errorf("BuildInstForBeaconReward error %+v\n Totalreward: %+v, epoch: %+v, reward: %+v\n", err, totalReward, epoch, baseRewards)
			return nil, err
		}
		resInst = append(resInst, singleInst)
	}
	return resInst, nil
}

func (beaconView *BeaconView) buildInstRewardForShards(epoch uint64, totalRewards []map[common.Hash]uint64) ([][]string, error) {
	resInst := [][]string{}
	for i, reward := range totalRewards {
		if len(reward) > 0 {
			shardRewardInst, err := metadata.BuildInstForShardReward(reward, epoch, byte(i))
			if err != nil {
				Logger.log.Errorf("BuildInstForShardReward error %+v\n Totalreward: %+v, epoch: %+v\n; shard:%+v", err, reward, epoch, byte(i))
				return nil, err
			}
			resInst = append(resInst, shardRewardInst...)
		}
	}
	return resInst, nil
}

func (beaconView *BeaconView) buildInstRewardForIncDAO(epoch uint64, totalReward map[common.Hash]uint64) ([][]string, error) {
	resInst := [][]string{}
	devRewardInst, err := metadata.BuildInstForIncDAOReward(totalReward, beaconView.bc.chainParams.IncognitoDAOAddress)
	if err != nil {
		Logger.log.Errorf("buildInstRewardForIncDAO error %+v\n Totalreward: %+v, epoch: %+v\n", err, totalReward, epoch)
		return nil, err
	}
	resInst = append(resInst, devRewardInst)
	return resInst, nil
}

func (bca *BeaconCoreApp) buildAssignInstruction() (err error) {
	instructions := [][]string{}
	numberOfPendingValidator := make(map[byte]int)
	for i := 0; i < bca.bc.GetActiveShard(); i++ {
		if pendingValidators, ok := bca.CreateState.curView.ShardPendingValidator[byte(i)]; ok {
			numberOfPendingValidator[byte(i)] = len(pendingValidators)
		} else {
			numberOfPendingValidator[byte(i)] = 0
		}
	}

	shardCandidatesStr, err := incognitokey.CommitteeKeyListToString(bca.CreateState.curView.CandidateShardWaitingForCurrentRandom)
	if err != nil {
		panic(err)
	}
	_, assignedCandidates := assignShardCandidate(shardCandidatesStr, numberOfPendingValidator, bca.CreateState.randomNumber, bca.bc.chainParams.AssignOffset, bca.bc.GetActiveShard())
	var keys []int
	for k := range assignedCandidates {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, key := range keys {
		shardID := byte(key)
		candidates := assignedCandidates[shardID]
		Logger.log.Infof("Assign Candidate at Shard %+v: %+v", shardID, candidates)
		shardAssingInstruction := []string{AssignAction}
		shardAssingInstruction = append(shardAssingInstruction, strings.Join(candidates, ","))
		shardAssingInstruction = append(shardAssingInstruction, "shard")
		shardAssingInstruction = append(shardAssingInstruction, fmt.Sprintf("%v", shardID))
		instructions = append(instructions, shardAssingInstruction)
	}
	bca.CreateState.shardAssignInst = instructions

	return nil
}

func (bca *BeaconCoreApp) buildRandomInstruction() (err error) {
	var chainTimeStamp int64
	curView := bca.CreateState.curView

	if curView.IsGettingRandomNumber {
		chainTimeStamp, err = bca.bc.randomClient.GetCurrentChainTimeStamp()
		if chainTimeStamp > curView.CurrentRandomTimeStamp {

			randomInstruction, rand, err := bca.generateRandomInstruction(bca.bc.randomClient)
			if err != nil {
				return err
			}

			bca.CreateState.randomInstruction = randomInstruction
			bca.CreateState.randomNumber = rand
			Logger.log.Infof("Beacon Producer found Random Instruction at Block Height %+v, %+v", randomInstruction, curView.GetHeight()+1)
		}
	}
	return nil
}

// ["random" "{nonce}" "{blockheight}" "{timestamp}" "{bitcoinTimestamp}"]
func (bca *BeaconCoreApp) generateRandomInstruction(randomClient btc.RandomClient) ([]string, int64, error) {
	curView := bca.CreateState.curView
	timestamp := curView.CurrentRandomTimeStamp
	var (
		blockHeight    int
		chainTimestamp int64
		nonce          int64
		strs           []string
		err            error
	)
	startTime := time.Now()
	blockHeight, chainTimestamp, nonce, err = randomClient.GetNonceByTimestamp(startTime, time.Second*10, timestamp)
	if err != nil {
		return strs, nonce, err
	}
	strs = append(strs, "random")
	strs = append(strs, strconv.Itoa(int(nonce)))
	strs = append(strs, strconv.Itoa(blockHeight))
	strs = append(strs, strconv.Itoa(int(timestamp)))
	strs = append(strs, strconv.Itoa(int(chainTimestamp)))
	return strs, int64(nonce), nil
}

func (beaconView *BeaconView) buildChangeBeaconValidatorByEpoch() (instructions [][]string, err error) {
	swapBeaconInstructions := []string{}

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconPendingValidator)
	if err != nil {
		return
	}
	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(beaconView.BeaconCommittee)
	if err != nil {
		return
	}

	producersBlackList, err := getUpdatedProducersBlackList(true, -1, beaconCommitteeStr, beaconView.GetHeight(), beaconView.bc, beaconView.GetCopiedSlashStateDB())
	if err != nil {
		Logger.log.Error(err)
	}

	badProducersWithPunishment := buildBadProducersWithPunishment(true, -1, beaconCommitteeStr, beaconView.bc)
	badProducersWithPunishmentBytes, err := json.Marshal(badProducersWithPunishment)
	if err != nil {
		Logger.log.Error(err)
	}

	_, _, swappedValidator, beaconNextCommittee, err := SwapValidator(beaconPendingValidatorStr, beaconCommitteeStr, beaconView.bc.chainParams.MaxBeaconCommitteeSize, beaconView.bc.chainParams.MinBeaconCommitteeSize, beaconView.bc.chainParams.Offset, producersBlackList, beaconView.bc.chainParams.SwapOffset)

	if len(swappedValidator) > 0 || len(beaconNextCommittee) > 0 && err == nil {
		swapBeaconInstructions = append(swapBeaconInstructions, "swap")
		swapBeaconInstructions = append(swapBeaconInstructions, strings.Join(beaconNextCommittee, ","))
		swapBeaconInstructions = append(swapBeaconInstructions, strings.Join(swappedValidator, ","))
		swapBeaconInstructions = append(swapBeaconInstructions, "beacon")
		swapBeaconInstructions = append(swapBeaconInstructions, string(badProducersWithPunishmentBytes))
		instructions = append(instructions, swapBeaconInstructions)
	}

	return
}

func (beaconView *BeaconView) buildRewardInstructionByEpoch() ([][]string, error) {
	var resInst [][]string
	var instRewardForBeacons [][]string
	var instRewardForIncDAO [][]string
	var instRewardForShards [][]string
	var err error
	numberOfActiveShards := beaconView.bc.GetActiveShard()
	totalRewards := make([]map[common.Hash]uint64, numberOfActiveShards)
	totalRewardForBeacon := map[common.Hash]uint64{}
	totalRewardForIncDAO := map[common.Hash]uint64{}
	epoch := beaconView.GetEpoch()
	rewardStateDB := beaconView.GetCopiedRewardStateDB()
	allCoinID := statedb.GetAllTokenIDForReward(rewardStateDB, epoch)
	blkPerYear := getNoBlkPerYear(uint64(beaconView.bc.chainParams.MaxBeaconBlockCreation.Seconds()))
	percentForIncognitoDAO := getPercentForIncognitoDAO(beaconView.GetHeight(), blkPerYear)
	for ID := 0; ID < numberOfActiveShards; ID++ {
		if totalRewards[ID] == nil {
			totalRewards[ID] = map[common.Hash]uint64{}
		}
		for _, coinID := range allCoinID {
			totalRewards[ID][coinID], err = statedb.GetRewardOfShardByEpoch(rewardStateDB, epoch, byte(ID), coinID)
			if err != nil {
				return nil, err
			}
			if totalRewards[ID][coinID] == 0 {
				delete(totalRewards[ID], coinID)
			}
		}
		rewardForBeacon, rewardForIncDAO, err := splitReward(&totalRewards[ID], numberOfActiveShards, percentForIncognitoDAO)
		if err != nil {
			beaconView.Logger.Infof("\n------------------------------------\nNot enough reward in epoch %v\n------------------------------------\n", err)
		}
		mapPlusMap(rewardForBeacon, &totalRewardForBeacon)
		mapPlusMap(rewardForIncDAO, &totalRewardForIncDAO)
	}
	if len(totalRewardForBeacon) > 0 {
		instRewardForBeacons, err = beaconView.buildInstRewardForBeacons(epoch, totalRewardForBeacon)
		if err != nil {
			return nil, err
		}
	}

	instRewardForShards, err = beaconView.buildInstRewardForShards(epoch, totalRewards)
	if err != nil {
		return nil, err
	}

	if len(totalRewardForIncDAO) > 0 {
		instRewardForIncDAO, err = beaconView.buildInstRewardForIncDAO(epoch, totalRewardForIncDAO)
		if err != nil {
			return nil, err
		}
	}
	resInst = common.AppendSliceString(instRewardForBeacons, instRewardForIncDAO, instRewardForShards)
	return resInst, nil
}

func getNoBlkPerYear(blockCreationTimeSeconds uint64) uint64 {
	//31536000 =
	return (365 * 24 * 60 * 60) / blockCreationTimeSeconds
}

func getPercentForIncognitoDAO(blockHeight, blkPerYear uint64) int {
	year := blockHeight / blkPerYear
	if year > (params.UpperBoundPercentForIncDAO - params.LowerBoundPercentForIncDAO) {
		return params.LowerBoundPercentForIncDAO
	} else {
		return params.UpperBoundPercentForIncDAO - int(year)
	}
}

// mapPlusMap(src, dst): dst = dst + src
func mapPlusMap(src, dst *map[common.Hash]uint64) {
	if src != nil {
		for key, value := range *src {
			(*dst)[key] += value
		}
	}
}

// calculateMapReward(src, dst): dst = dst + src
func splitReward(
	totalReward *map[common.Hash]uint64,
	numberOfActiveShards int,
	devPercent int,
) (
	*map[common.Hash]uint64,
	*map[common.Hash]uint64,
	error,
) {
	hasValue := false
	rewardForBeacon := map[common.Hash]uint64{}
	rewardForIncDAO := map[common.Hash]uint64{}
	for key, value := range *totalReward {
		rewardForBeacon[key] = 2 * (uint64(100-devPercent) * value) / ((uint64(numberOfActiveShards) + 2) * 100)
		rewardForIncDAO[key] = uint64(devPercent) * value / uint64(100)
		(*totalReward)[key] = value - (rewardForBeacon[key] + rewardForIncDAO[key])
		if !hasValue {
			hasValue = true
		}
	}
	if !hasValue {
		//fmt.Printf("[ndh] not enough reward\n")
		return nil, nil, NewBlockChainError(NotEnoughRewardError, errors.New("Not enough reward"))
	}
	return &rewardForBeacon, &rewardForIncDAO, nil
}

const (
	RandomInst        = "RandomInst"
	StopAutoStakeInst = "StopAutoStakeInst"
	ShardSwapInst     = "ShardSwapInst"
	BeaconSwapInst    = "BeaconSwapInst"
	BeaconStakeInst   = "BeaconStakeInst"
	ShardStakeInst    = "ShardStakeInst"
	ShardAssignInst   = "ShardAssignInst"
)

func instructionType(instruction []string) string {
	if instruction[0] == RandomAction {
		return RandomInst
	}
	if instruction[0] == StopAutoStake {
		return StopAutoStakeInst
	}
	if instruction[0] == SwapAction {
		if instruction[3] == "shard" {
			return ShardSwapInst
		}
		if instruction[3] == "beacon" {
			return BeaconSwapInst
		}
	}
	if instruction[0] == StakeAction && instruction[2] == "beacon" {
		return BeaconStakeInst
	}
	if instruction[0] == StakeAction && instruction[2] == "shard" {
		return ShardStakeInst
	}

	if instruction[0] == AssignAction && instruction[2] == "shard" {
		return ShardAssignInst
	}

	return ""
}

//from beacon
func extractRandomInst(instruction []string) (rand int64, err error) {
	temp, err := strconv.Atoi(instruction[1])
	if err != nil {
		return 0, err
	}
	return int64(temp), nil
}

func extractStopAutoStakeInst(instruction []string) (stopAutoCommittees []string) {
	committeePublicKeys := strings.Split(instruction[1], ",")
	return committeePublicKeys
}

func extractBeaconSwapInst(instruction []string) (inPublicKeys []string, outPublicKeys []string) {
	inPublickeys := strings.Split(instruction[1], ",")
	outPublickeys := strings.Split(instruction[2], ",")
	return inPublickeys, outPublickeys
}

func extractShardSwapInst(instruction []string) (inPublicKeys []string, outPublicKeys []string, shardID byte, err error) {
	inPublickeys := strings.Split(instruction[1], ",")
	outPublickeys := strings.Split(instruction[2], ",")
	temp, err := strconv.Atoi(instruction[4])
	if err != nil {
		return nil, nil, 0, err
	}
	shardID = byte(temp)
	return inPublickeys, outPublickeys, shardID, nil
}

func extractBeaconStakeInst(instruction []string) (beaconCandidates []string, stakeTx []string, beaconRewardReceivers []string, beaconAutoReStaking []string) {
	beaconCandidates = strings.Split(instruction[1], ",")
	stakeTx = strings.Split(instruction[3], ",")
	beaconRewardReceivers = strings.Split(instruction[4], ",")
	beaconAutoReStaking = strings.Split(instruction[5], ",")
	return
}

func extractShardStakeInst(instruction []string) (shardCandidates []string, stakeTx []string, shardRewardReceivers []string, shardAutoReStaking []string) {
	shardCandidates = strings.Split(instruction[1], ",")
	stakeTx = strings.Split(instruction[3], ",")
	shardRewardReceivers = strings.Split(instruction[4], ",")
	shardAutoReStaking = strings.Split(instruction[5], ",")
	return
}

func extractAssignInst(instruction []string) (assignPubkeys []string, shardID string) {
	assignPubkeys = strings.Split(instruction[1], ",")
	shardID = instruction[3]
	return
}

//From Shard
func extractShardSwapInstFromShard(instruction []string) (inPublickeys []string, outPublickeys []string) {
	inPublickeys = strings.Split(instruction[1], ",")
	outPublickeys = strings.Split(instruction[2], ",")
	return
}

func extractShardStateFromShardBlocks(s2bBlks map[byte][]blockinterface.ShardToBeaconBlockInterface) map[byte][]shardstate.ShardState {

	shardStates := make(map[byte][]shardstate.ShardState)
	for shardID, shardBlocks := range s2bBlks {
		for _, s2bBlk := range shardBlocks {
			shardState := shardstate.ShardState{}
			header := s2bBlk.GetShardHeader()
			shardState.CrossShard = make([]byte, len(header.GetCrossShardBitMap()))
			copy(shardState.CrossShard, header.GetCrossShardBitMap())
			shardState.Hash = *header.GetHash()
			shardState.Height = header.GetHeight()
			shardStates[shardID] = append(shardStates[shardID], shardState)
		}
	}
	return shardStates
}
