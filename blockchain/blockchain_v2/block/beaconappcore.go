package block

import (
	"errors"
	"fmt"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"

	"time"
)

type BeaconCoreApp struct {
	Logger        common.Logger
	CreateState   *CreateBeaconBlockState
	ValidateState *ValidateBeaconBlockState
}

func (s *BeaconCoreApp) preCreateBlock() error {
	state := s.CreateState
	curView := state.curView

	//TODO: get s2b blocks from pool => s2bBlks
	// -> Only accept block in one epoch
	state.s2bBlks = make(map[byte][]*ShardToBeaconBlock)

	//newEpoch? endEpoch? finalBlockInEpoch?
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == 1 {
		state.isNewEpoch = true
	}
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == 0 {
		state.isEndEpoch = true
	}
	if (curView.GetHeight()+1)%curView.BC.GetChainParams().Epoch == curView.BC.GetChainParams().RandomTime {
		state.isRandomTime = true
	}

	//shardstates
	shardStates := make(map[byte][]ShardState)
	for shardID, shardBlocks := range state.s2bBlks {
		for _, s2bBlk := range shardBlocks {
			shardState := ShardState{}
			shardState.CrossShard = make([]byte, len(s2bBlk.Header.CrossShardBitMap))
			copy(shardState.CrossShard, s2bBlk.Header.CrossShardBitMap)
			shardState.Hash = s2bBlk.Header.Hash()
			shardState.Height = s2bBlk.Header.Height
			shardStates[shardID] = append(shardStates[shardID], shardState)
		}
	}
	s.CreateState.shardStates = shardStates
	return nil
}

func (s *BeaconCoreApp) buildInstructionByEpoch() error {
	state := s.CreateState
	curView := state.curView

	//build reward instruction
	if state.isNewEpoch {
		var err error
		if state.rewardInstByEpoch, err = curView.getRewardInstByEpoch(); err != nil {
			return blockchain.NewBlockChainError(blockchain.BuildRewardInstructionError, err)
		}
	}

	//build beacon committee change instruction
	if state.isEndEpoch {
		var err error
		if state.beaconSwapInstruction, err = curView.buildChangeBeaconValidatorByEpoch(); err != nil {
			panic(err)
		}
	}

	//build random instruction
	if curView.IsGettingRandomNumber {
		if err := s.buildRandomInstruction(); err != nil {
			panic(err)
		}
	}

	//build assign instruction - if getting random number and get one random
	if curView.IsGettingRandomNumber && len(state.randomInstruction) >= 1 {
		if err := s.buildAsssignInstruction(); err != nil {
			panic(err)
		}
	}
	return nil
}

func (s *BeaconCoreApp) buildInstructionFromShardAction() error {
	state := s.CreateState
	curView := state.curView

	//build staking & auto staking instruction & shard committee instruction
	validStakeInstructions := [][]string{}
	validStakePublicKeys := []string{}
	validStopAutoStakingInstructions := [][]string{}
	validSwapInstructions := make(map[byte][][]string)
	acceptedRewardInstructions := [][]string{}

	for shardID, shardBlocks := range state.s2bBlks {
		Logger.log.Infof("Beacon Producer Got %+v Shard Block from shard %+v: ", len(shardBlocks), shardID)
		for _, shardBlock := range shardBlocks {
			validStakeInstruction, tempValidStakePublicKeys, validSwapInstruction, acceptedRewardInstruction, stopAutoStakingInstruction := buildInstructionFromBlock(shardBlock, curView)

			validStakeInstructions = append(validStakeInstructions, validStakeInstruction...)
			validSwapInstructions[shardID] = append(validSwapInstructions[shardID], validSwapInstruction[shardID]...)
			acceptedRewardInstructions = append(acceptedRewardInstructions, acceptedRewardInstruction)
			validStopAutoStakingInstructions = append(validStopAutoStakingInstructions, stopAutoStakingInstruction...)
			validStakePublicKeys = append(validStakePublicKeys, tempValidStakePublicKeys...)
		}
	}

	s.CreateState.validStakeInstructions = validStakeInstructions
	s.CreateState.validSwapInstructions = validSwapInstructions
	s.CreateState.acceptedRewardInstructions = acceptedRewardInstructions
	s.CreateState.validStopAutoStakingInstructions = validStopAutoStakingInstructions
	return nil
}

func (s *BeaconCoreApp) buildHeader() error {
	curView := s.CreateState.curView

	newBlock := s.CreateState.newBlock
	newBlock.Header = BeaconHeader{}

	//======Build Header Essential Data=======
	newBlock.Header.Version = blockchain.BEACON_BLOCK_VERSION
	newBlock.Header.Height = curView.GetHeight() + 1
	if s.CreateState.isNewEpoch {
		newBlock.Header.Epoch = curView.GetEpoch() + 1
	}
	newBlock.Header.ConsensusType = common.BlsConsensus2
	newBlock.Header.Producer = s.CreateState.proposer
	newBlock.Header.ProducerPubKeyStr = s.CreateState.proposer

	newBlock.Header.Timestamp = s.CreateState.createTimeStamp
	newBlock.Header.TimeSlot = s.CreateState.createTimeSlot
	newBlock.Header.PreviousBlockHash = *curView.GetBlock().Hash()

	//============Build Header Hash=============
	// create new view
	newViewInterface, err := curView.CreateNewViewFromBlock(newBlock)
	if err != nil {
		return err
	}
	newView := newViewInterface.(*BeaconView)
	// BeaconValidator root: beacon committee + beacon pending committee
	beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconCommittee)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	validatorArr := append([]string{}, beaconCommitteeStr...)

	beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconPendingValidator)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	validatorArr = append(validatorArr, beaconPendingValidatorStr...)
	tempBeaconCommitteeAndValidatorRoot, err := GenerateHashFromStringArray(validatorArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateBeaconCommitteeAndValidatorRootError, err)
	}
	// BeaconCandidate root: beacon current candidate + beacon next candidate
	beaconCandidateArr := append(newView.CandidateBeaconWaitingForCurrentRandom, newView.CandidateBeaconWaitingForNextRandom...)

	beaconCandidateArrStr, err := incognitokey.CommitteeKeyListToString(beaconCandidateArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	tempBeaconCandidateRoot, err := GenerateHashFromStringArray(beaconCandidateArrStr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateBeaconCandidateRootError, err)
	}
	// Shard candidate root: shard current candidate + shard next candidate
	shardCandidateArr := append(newView.CandidateShardWaitingForCurrentRandom, newView.CandidateShardWaitingForNextRandom...)

	shardCandidateArrStr, err := incognitokey.CommitteeKeyListToString(shardCandidateArr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
	}
	tempShardCandidateRoot, err := GenerateHashFromStringArray(shardCandidateArrStr)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateShardCandidateRootError, err)
	}
	// Shard Validator root
	shardPendingValidator := make(map[byte][]string)
	for shardID, keys := range newView.ShardPendingValidator {
		keysStr, err := incognitokey.CommitteeKeyListToString(keys)
		if err != nil {
			return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
		}
		shardPendingValidator[shardID] = keysStr
	}

	shardCommittee := make(map[byte][]string)
	for shardID, keys := range newView.ShardCommittee {
		keysStr, err := incognitokey.CommitteeKeyListToString(keys)
		if err != nil {
			return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
		}
		shardCommittee[shardID] = keysStr
	}

	tempShardCommitteeAndValidatorRoot, err := GenerateHashFromMapByteString(shardPendingValidator, shardCommittee)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.GenerateShardCommitteeAndValidatorRootError, err)
	}

	tempAutoStakingRoot, err := GenerateHashFromMapStringBool(newView.AutoStaking)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.AutoStakingRootHashError, err)
	}
	// Shard state hash
	tempShardStateHash, err := GenerateHashFromShardState(s.CreateState.shardStates)
	if err != nil {
		Logger.log.Error(err)
		return blockchain.NewBlockChainError(blockchain.GenerateShardStateError, err)
	}
	// Instruction Hash
	tempInstructionArr := []string{}
	for _, strs := range s.CreateState.newBlock.Body.Instructions {
		tempInstructionArr = append(tempInstructionArr, strs...)
	}
	tempInstructionHash, err := GenerateHashFromStringArray(tempInstructionArr)
	if err != nil {
		Logger.log.Error(err)
		return blockchain.NewBlockChainError(blockchain.GenerateInstructionHashError, err)
	}
	// Instruction merkle root
	flattenInsts, err := blockchain.FlattenAndConvertStringInst(s.CreateState.newBlock.Body.Instructions)
	if err != nil {
		return blockchain.NewBlockChainError(blockchain.FlattenAndConvertStringInstError, err)
	}
	// add hash to header
	newBlock.Header.BeaconCommitteeAndValidatorRoot = tempBeaconCommitteeAndValidatorRoot
	newBlock.Header.BeaconCandidateRoot = tempBeaconCandidateRoot
	newBlock.Header.ShardCandidateRoot = tempShardCandidateRoot
	newBlock.Header.ShardCommitteeAndValidatorRoot = tempShardCommitteeAndValidatorRoot
	newBlock.Header.ShardStateHash = tempShardStateHash
	newBlock.Header.InstructionHash = tempInstructionHash
	newBlock.Header.AutoStakingRoot = tempAutoStakingRoot
	copy(newBlock.Header.InstructionMerkleRoot[:], blockchain.GetKeccak256MerkleRoot(flattenInsts))
	newBlock.Header.Timestamp = time.Now().Unix()
	return nil
}

func (s *BeaconCoreApp) updateNewViewFromBlock(block *BeaconBlock) (err error) {
	s.CreateState.newView.Block = block
	newView := s.CreateState.newView
	//curView := s.CreateState.curView
	newShardCandidates := []incognitokey.CommitteePublicKey{}
	newBeaconCandidates := []incognitokey.CommitteePublicKey{}
	randomFlag := false
	for _, inst := range block.Body.Instructions {
		switch instructionType(inst) {
		case RandomInst:
			if newView.IsGettingRandomNumber { //only process if in getting random number
				newView.CurrentRandomNumber, err = extractRandomInst(inst)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessRandomInstructionError, err)
				}
				randomFlag = true
			}
		case StopAutoStakeInst:
			stopAutoCommittees := extractStopAutoStakeInst(inst)
			for _, committeePublicKey := range stopAutoCommittees {
				allCommitteeValidatorCandidate := newView.getAllCommitteeValidatorCandidateFlattenList()
				// check existence in all committee list
				if common.IndexOfStr(committeePublicKey, allCommitteeValidatorCandidate) == -1 {
					// if not found then delete auto staking data for this public key if present
					if _, ok := newView.AutoStaking[committeePublicKey]; ok {
						delete(newView.AutoStaking, committeePublicKey)
					}
				} else {
					// if found in committee list then turn off auto staking
					if _, ok := newView.AutoStaking[committeePublicKey]; ok {
						newView.AutoStaking[committeePublicKey] = false
					}
				}
			}
		case ShardSwapInst:
			in, out, shardID, err := extractShardSwapInst(inst)
			if err != nil {
				return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
			}

			// delete in public key out of sharding pending validator list
			if len(in) > 0 {
				shardPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(newView.ShardPendingValidator[shardID])
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				tempShardPendingValidator, err := RemoveValidator(shardPendingValidatorStr, in)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessSwapInstructionError, err)
				}
				// update shard pending validator
				newView.ShardPendingValidator[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardPendingValidator)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessSwapInstructionError, err)
				}
				// add new public key to committees
				inPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(in)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				newView.ShardCommittee[shardID] = append(newView.ShardCommittee[shardID], inPublickeyStructs...)
			}

			// delete out public key out of current committees
			if len(out) > 0 {
				outPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(out)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}

				shardCommitteeStr, err := incognitokey.CommitteeKeyListToString(newView.ShardCommittee[shardID])
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				tempShardCommittees, err := RemoveValidator(shardCommitteeStr, out)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessSwapInstructionError, err)
				}
				// remove old public key in shard committee update shard committee
				newView.ShardCommittee[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardCommittees)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				// Check auto stake in out public keys list
				// if auto staking not found or flag auto stake is false then do not re-stake for this out public key
				// if auto staking flag is true then system will automatically add this out public key to current candidate list
				for index, outPublicKey := range out {
					if isAutoRestaking, ok := newView.AutoStaking[outPublicKey]; !ok {
						if _, ok := newView.RewardReceiver[outPublicKey]; ok {
							delete(newView.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoRestaking {
							// delete this flag for next time staking
							delete(newView.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
							delete(newView.AutoStaking, outPublicKey)
						} else {
							shardCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
							}
							newShardCandidates = append(newShardCandidates, shardCandidate...)
						}
					}
				}
			}

		case BeaconSwapInst:
			in, out, err := extractBeaconSwapInst(inst)
			if err != nil {
				return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
			}
			if len(in) > 0 {
				beaconPendingValidatorStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconPendingValidator)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				tempBeaconPendingValidator, err := RemoveValidator(beaconPendingValidatorStr, in)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessSwapInstructionError, err)
				}
				// update beacon pending validator
				newView.BeaconPendingValidator, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconPendingValidator)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				// add new public key to beacon committee
				inPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(in)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				newView.BeaconCommittee = append(newView.BeaconCommittee, inPublickeyStructs...)
			}

			if len(out) > 0 {
				outPublickeyStructs, err := incognitokey.CommitteeBase58KeyListToStruct(out)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				beaconCommitteeStr, err := incognitokey.CommitteeKeyListToString(newView.BeaconCommittee)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				tempBeaconCommittes, err := RemoveValidator(beaconCommitteeStr, out)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.ProcessSwapInstructionError, err)
				}
				// remove old public key in beacon committee and update beacon best state
				newView.BeaconCommittee, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconCommittes)
				if err != nil {
					return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
				}
				for index, outPublicKey := range out {
					if isAutoRestaking, ok := newView.AutoStaking[outPublicKey]; !ok {
						if _, ok := newView.RewardReceiver[outPublicKey]; ok {
							delete(newView.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoRestaking {
							delete(newView.RewardReceiver, outPublickeyStructs[index].GetIncKeyBase58())
							delete(newView.AutoStaking, outPublicKey)
						} else {
							beaconCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
							}
							newBeaconCandidates = append(newBeaconCandidates, beaconCandidate...)
						}
					}
				}
			}
		case BeaconStakeInst:
			beaconCandidates, beaconRewardReceivers, beaconAutoReStaking := extractBeaconStakeInst(inst)
			beaconCandidatesStructs, err := incognitokey.CommitteeBase58KeyListToStruct(beaconCandidates)
			if err != nil {
				return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
			}
			if len(beaconCandidatesStructs) != len(beaconRewardReceivers) && len(beaconRewardReceivers) != len(beaconAutoReStaking) {
				return blockchain.NewBlockChainError(blockchain.StakeInstructionError, fmt.Errorf("Expect Beacon Candidate (length %+v) and Beacon Reward Receiver (length %+v) and Beacon Auto ReStaking (lenght %+v) have equal length"))
			}
			for index, candidate := range beaconCandidatesStructs {
				newView.RewardReceiver[candidate.GetIncKeyBase58()] = beaconRewardReceivers[index]
				if beaconAutoReStaking[index] == "true" {
					newView.AutoStaking[beaconCandidates[index]] = true
				} else {
					newView.AutoStaking[beaconCandidates[index]] = false
				}
			}
			newBeaconCandidates = append(newBeaconCandidates, beaconCandidatesStructs...)
			return nil
		case ShardStakeInst:
			shardCandidates, shardRewardReceivers, shardAutoReStaking := extractShardStakeInst(inst)
			shardCandidatesStructs, err := incognitokey.CommitteeBase58KeyListToStruct(shardCandidates)
			if err != nil {
				return blockchain.NewBlockChainError(blockchain.UnExpectedError, err)
			}
			if len(shardCandidates) != len(shardRewardReceivers) && len(shardRewardReceivers) != len(shardAutoReStaking) {
				return blockchain.NewBlockChainError(blockchain.StakeInstructionError, fmt.Errorf("Expect Beacon Candidate (length %+v) and Beacon Reward Receiver (length %+v) and Shard Auto ReStaking (length %+v) have equal length"))
			}
			for index, candidate := range shardCandidatesStructs {
				newView.RewardReceiver[candidate.GetIncKeyBase58()] = shardRewardReceivers[index]
				if shardAutoReStaking[index] == "true" {
					newView.AutoStaking[shardCandidates[index]] = true
				} else {
					newView.AutoStaking[shardCandidates[index]] = false
				}
			}
			newShardCandidates = append(newShardCandidates, shardCandidatesStructs...)
		default:
			return errors.New("Unknown Instruction")
		}
	}

	//process newShardCandidates, newBeaconCandidates
	// update candidate list after processing instructions
	newView.CandidateBeaconWaitingForNextRandom = append(newView.CandidateBeaconWaitingForNextRandom, newBeaconCandidates...)
	newView.CandidateShardWaitingForNextRandom = append(newView.CandidateShardWaitingForNextRandom, newShardCandidates...)

	if s.CreateState.isEndEpoch {
		// Begin of each epoch
		newView.IsGettingRandomNumber = false
		// Before get random from bitcoin
	}

	if s.CreateState.isRandomTime {
		newView.IsGettingRandomNumber = true
		// snapshot candidate list
		newView.CandidateShardWaitingForCurrentRandom = append(newView.CandidateShardWaitingForCurrentRandom, newView.CandidateShardWaitingForNextRandom...)
		newView.CandidateBeaconWaitingForCurrentRandom = append(newView.CandidateBeaconWaitingForCurrentRandom, newView.CandidateBeaconWaitingForNextRandom...)
		Logger.log.Info("Beacon Process: CandidateShardWaitingForCurrentRandom: ", newView.CandidateShardWaitingForCurrentRandom)
		Logger.log.Info("Beacon Process: CandidateBeaconWaitingForCurrentRandom: ", newView.CandidateBeaconWaitingForCurrentRandom)
		// reset candidate list
		newView.CandidateShardWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
		newView.CandidateBeaconWaitingForNextRandom = []incognitokey.CommitteePublicKey{}
		// assign random timestamp
		newView.CurrentRandomTimeStamp = s.CreateState.createTimeStamp
	}

	// if get new random number
	// Assign candidate to shard
	// assign CandidateShardWaitingForCurrentRandom to ShardPendingValidator with CurrentRandom
	if randomFlag {
		numberOfPendingValidator := make(map[byte]int)
		for shardID, pendingValidators := range newView.ShardPendingValidator {
			numberOfPendingValidator[shardID] = len(pendingValidators)
		}

		shardCandidatesStr, err := incognitokey.CommitteeKeyListToString(newView.CandidateShardWaitingForCurrentRandom)
		if err != nil {
			panic(err)
		}

		remainShardCandidatesStr, assignedCandidates := assignShardCandidate(shardCandidatesStr, numberOfPendingValidator, newView.CurrentRandomNumber, newView.BC.GetChainParams().AssignOffset, newView.GetActiveShard())
		remainShardCandidates, err := incognitokey.CommitteeBase58KeyListToStruct(remainShardCandidatesStr)
		if err != nil {
			panic(err)
		}

		// append remain candidate into shard waiting for next random list
		newView.CandidateShardWaitingForNextRandom = append(newView.CandidateShardWaitingForNextRandom, remainShardCandidates...)
		// assign candidate into shard pending validator list
		for shardID, candidateListStr := range assignedCandidates {
			candidateList, err := incognitokey.CommitteeBase58KeyListToStruct(candidateListStr)
			if err != nil {
				panic(err)
			}
			newView.ShardPendingValidator[shardID] = append(newView.ShardPendingValidator[shardID], candidateList...)
		}

		// delete CandidateShardWaitingForCurrentRandom list
		newView.CandidateShardWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
		// Shuffle candidate
		// shuffle CandidateBeaconWaitingForCurrentRandom with current random number
		newBeaconPendingValidator, err := ShuffleCandidate(newView.CandidateBeaconWaitingForCurrentRandom, newView.CurrentRandomNumber)
		if err != nil {
			return blockchain.NewBlockChainError(blockchain.ShuffleBeaconCandidateError, err)
		}
		newView.CandidateBeaconWaitingForCurrentRandom = []incognitokey.CommitteePublicKey{}
		newView.BeaconPendingValidator = append(newView.BeaconPendingValidator, newBeaconPendingValidator...)
	}

	return nil
}

func (BeaconCoreApp) preValidate() error {
	return nil
}
