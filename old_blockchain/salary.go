package blockchain

import (
	"fmt"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/pkg/errors"
)

func (blockGenerator *BlockGenerator) buildReturnStakingAmountTx(swapPublicKey string, blkProducerPrivateKey *privacy.PrivateKey, shardID byte) (metadata.Transaction, error) {
	publicKey, _ := blockGenerator.chain.config.ConsensusEngine.GetCurrentMiningPublicKey()
	_, committeeShardID := blockGenerator.chain.BestState.Beacon.GetPubkeyRole(publicKey, 0)
	Logger.log.Infof("Return Staking Amount public key %+v, staking transaction hash %+v, shardID %+v", swapPublicKey, GetBestStateShard(committeeShardID).StakingTx, committeeShardID)
	tx, ok := GetBestStateShard(committeeShardID).StakingTx[swapPublicKey]
	if !ok {
		return nil, NewBlockChainError(GetStakingTransactionError, errors.New("No staking tx in best state"))
	}
	var txHash = &common.Hash{}
	err := (&common.Hash{}).Decode(txHash, tx)
	if err != nil {
		return nil, NewBlockChainError(DecodeHashError, err)
	}
	blockHash, index, err := rawdbv2.GetTransactionByHash(blockGenerator.chain.config.DataBase, *txHash)
	if err != nil {
		return nil, NewBlockChainError(GetTransactionFromDatabaseError, err)
	}
	shardBlock, _, err := blockGenerator.chain.GetShardBlockByHash(blockHash)
	if err != nil || shardBlock == nil {
		Logger.log.Error("ERROR", err, "NO Transaction in block with hash", blockHash, "and index", index, "contains", shardBlock.Body.Transactions[index])
		return nil, NewBlockChainError(FetchShardBlockError, err)
	}
	txData := shardBlock.Body.Transactions[index]
	keyWallet, err := wallet.Base58CheckDeserialize(txData.GetMetadata().(*metadata.StakingMetadata).FunderPaymentAddress)
	if err != nil {
		Logger.log.Error("SA: cannot get payment address", txData.GetMetadata().(*metadata.StakingMetadata), committeeShardID)
		return nil, NewBlockChainError(WalletKeySerializedError, err)
	}
	Logger.log.Info("SA: build salary tx", txData.GetMetadata().(*metadata.StakingMetadata).FunderPaymentAddress, committeeShardID)
	paymentShardID := common.GetShardIDFromLastByte(keyWallet.KeySet.PaymentAddress.Pk[len(keyWallet.KeySet.PaymentAddress.Pk)-1])
	if paymentShardID != committeeShardID {
		return nil, NewBlockChainError(WrongShardIDError, fmt.Errorf("Staking Payment Address ShardID %+v, Not From Current Shard %+v", paymentShardID, committeeShardID))
	}
	returnStakingMeta := metadata.NewReturnStaking(
		tx,
		keyWallet.KeySet.PaymentAddress,
		metadata.ReturnStakingMeta,
	)
	returnStakingTx := new(transaction.Tx)
	err = returnStakingTx.InitTxSalary(
		txData.CalculateTxValue(),
		&keyWallet.KeySet.PaymentAddress,
		blkProducerPrivateKey,
		blockGenerator.chain.BestState.Shard[shardID].GetCopiedTransactionStateDB(),
		returnStakingMeta,
	)
	//modify the type of the salary transaction
	returnStakingTx.Type = common.TxReturnStakingType
	if err != nil {
		return nil, NewBlockChainError(InitSalaryTransactionError, err)
	}
	return returnStakingTx, nil
}

func (blockchain *BlockChain) getRewardAmount(blkHeight uint64) uint64 {
	blockBeaconInterval := blockchain.config.ChainParams.MinBeaconBlockInterval.Seconds()
	blockInYear := getNoBlkPerYear(uint64(blockBeaconInterval))
	n := blkHeight / blockInYear
	reward := uint64(blockchain.config.ChainParams.BasicReward)
	for ; n > 0; n-- {
		reward *= 91
		reward /= 100
	}
	return reward
}

func (blockchain *BlockChain) addShardRewardRequestToBeacon(beaconBlock *BeaconBlock, rewardStateDB *statedb.StateDB) error {
	for _, inst := range beaconBlock.Body.Instructions {
		if len(inst) <= 2 {
			continue
		}
		if inst[0] == SetAction || inst[0] == StakeAction || inst[0] == RandomAction || inst[0] == SwapAction || inst[0] == AssignAction {
			continue
		}
		metaType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue
		}
		switch metaType {
		case metadata.AcceptedBlockRewardInfoMeta:
			acceptedBlkRewardInfo, err := metadata.NewAcceptedBlockRewardInfoFromStr(inst[2])
			if err != nil {
				return err
			}
			if val, ok := acceptedBlkRewardInfo.TxsFee[common.PRVCoinID]; ok {
				acceptedBlkRewardInfo.TxsFee[common.PRVCoinID] = val + blockchain.getRewardAmount(acceptedBlkRewardInfo.ShardBlockHeight)
			} else {
				if acceptedBlkRewardInfo.TxsFee == nil {
					acceptedBlkRewardInfo.TxsFee = map[common.Hash]uint64{}
				}
				acceptedBlkRewardInfo.TxsFee[common.PRVCoinID] = blockchain.getRewardAmount(acceptedBlkRewardInfo.ShardBlockHeight)
			}
			for key, value := range acceptedBlkRewardInfo.TxsFee {
				if value != 0 {
					err = statedb.AddShardRewardRequest(rewardStateDB, beaconBlock.Header.Epoch, acceptedBlkRewardInfo.ShardID, key, value)
					if err != nil {
						return err
					}
				}
			}
			continue
		}
	}
	return nil
}

func (blockchain *BlockChain) processSalaryInstructions(rewardStateDB *statedb.StateDB, beaconBlocks []*BeaconBlock, shardID byte) error {
	rewardReceivers := make(map[string]string)
	committees := make(map[int][]incognitokey.CommitteePublicKey)
	isInit := false
	epoch := uint64(0)
	for _, beaconBlock := range beaconBlocks {
		for _, l := range beaconBlock.Body.Instructions {
			if l[0] == StakeAction || l[0] == RandomAction {
				continue
			}
			if len(l) <= 2 {
				continue
			}
			shardToProcess, err := strconv.Atoi(l[1])
			if err != nil {
				continue
			}
			metaType, err := strconv.Atoi(l[0])
			if err != nil {
				return NewBlockChainError(ProcessSalaryInstructionsError, err)
			}
			if shardToProcess == int(shardID) {
				switch metaType {
				case metadata.BeaconRewardRequestMeta:
					beaconBlkRewardInfo, err := metadata.NewBeaconBlockRewardInfoFromStr(l[3])
					if err != nil {
						return NewBlockChainError(ProcessSalaryInstructionsError, err)
					}
					for key := range beaconBlkRewardInfo.BeaconReward {
						Logger.log.Criticalf("Add Committee Reward BeaconReward, Public Key %+v, reward %+v, token %+v", beaconBlkRewardInfo.PayToPublicKey, beaconBlkRewardInfo.BeaconReward[key], key)
						err = statedb.AddCommitteeReward(rewardStateDB, beaconBlkRewardInfo.PayToPublicKey, beaconBlkRewardInfo.BeaconReward[key], key)
						if err != nil {
							return NewBlockChainError(ProcessSalaryInstructionsError, err)
						}
					}
					continue

				case metadata.IncDAORewardRequestMeta:
					incDAORewardInfo, err := metadata.NewIncDAORewardInfoFromStr(l[3])
					if err != nil {
						return NewBlockChainError(ProcessSalaryInstructionsError, err)
					}
					keyWalletDevAccount, err := wallet.Base58CheckDeserialize(blockchain.config.ChainParams.IncognitoDAOAddress)
					if err != nil {
						return NewBlockChainError(ProcessSalaryInstructionsError, err)
					}
					for key := range incDAORewardInfo.IncDAOReward {
						tempPublicKey := base58.Base58Check{}.Encode(keyWalletDevAccount.KeySet.PaymentAddress.Pk, common.Base58Version)
						Logger.log.Criticalf("Add Committee Reward IncDAOReward, Public Key %+v, reward %+v, token %+v", tempPublicKey, incDAORewardInfo.IncDAOReward[key], key)
						err = statedb.AddCommitteeReward(rewardStateDB, tempPublicKey, incDAORewardInfo.IncDAOReward[key], key)
						if err != nil {
							return NewBlockChainError(ProcessSalaryInstructionsError, err)
						}
					}
					continue
				}
			}
			switch metaType {
			case metadata.ShardBlockRewardRequestMeta:
				shardRewardInfo, err := metadata.NewShardBlockRewardInfoFromString(l[3])
				if err != nil {
					return NewBlockChainError(ProcessSalaryInstructionsError, err)
				}
				if (!isInit) || (epoch != shardRewardInfo.Epoch) {
					isInit = true
					height := shardRewardInfo.Epoch * blockchain.config.ChainParams.Epoch
					consensusRootHash, err := blockchain.GetBeaconConsensusRootHash(blockchain.GetDatabase(), height)
					if err != nil {
						return NewBlockChainError(ProcessSalaryInstructionsError, fmt.Errorf("Beacon Consensus Root Hash of Height %+v not found ,error %+v", height, err))
					}
					consensusStateDB, err := statedb.NewWithPrefixTrie(consensusRootHash, statedb.NewDatabaseAccessWarper(blockchain.GetDatabase()))
					if err != nil {
						return NewBlockChainError(ProcessSalaryInstructionsError, err)
					}
					committees, rewardReceivers = statedb.GetAllCommitteeStateWithRewardReceiver(consensusStateDB, blockchain.GetShardIDs())
				}
				err = blockchain.addShardCommitteeReward(rewardStateDB, shardID, shardRewardInfo, committees[int(shardToProcess)], rewardReceivers)
				if err != nil {
					return err
				}
				continue
			}

		}
	}
	return nil
}

func (blockchain *BlockChain) addShardCommitteeReward(rewardStateDB *statedb.StateDB, shardID byte, rewardInfoShardToProcess *metadata.ShardBlockRewardInfo, committeeOfShardToProcess []incognitokey.CommitteePublicKey, rewardReceiver map[string]string) (err error) {
	committeeSize := len(committeeOfShardToProcess)
	for _, candidate := range committeeOfShardToProcess {
		wl, err := wallet.Base58CheckDeserialize(rewardReceiver[candidate.GetIncKeyBase58()])
		if err != nil {
			return NewBlockChainError(ProcessSalaryInstructionsError, err)
		}
		if common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[common.PublicKeySize-1]) == shardID {
			for key, value := range rewardInfoShardToProcess.ShardReward {
				tempPK := base58.Base58Check{}.Encode(wl.KeySet.PaymentAddress.Pk, common.Base58Version)
				Logger.log.Criticalf("Add Committee Reward ShardCommitteeReward, Public Key %+v, reward %+v, token %+v", tempPK, value/uint64(committeeSize), key)
				err = statedb.AddCommitteeReward(rewardStateDB, tempPK, value/uint64(committeeSize), key)
				if err != nil {
					return NewBlockChainError(ProcessSalaryInstructionsError, err)
				}
			}
		}
	}
	return nil
}

func (blockchain *BlockChain) buildRewardInstructionByEpoch(blkHeight, epoch uint64, rewardStateDB *statedb.StateDB) ([][]string, error) {
	var resInst [][]string
	var err error
	var instRewardForBeacons [][]string
	var instRewardForIncDAO [][]string
	var instRewardForShards [][]string
	numberOfActiveShards := blockchain.BestState.Beacon.ActiveShards
	allCoinID := statedb.GetAllTokenIDForReward(rewardStateDB, epoch)
	blkPerYear := getNoBlkPerYear(uint64(blockchain.config.ChainParams.MaxBeaconBlockCreation.Seconds()))
	percentForIncognitoDAO := getPercentForIncognitoDAO(blkHeight, blkPerYear)
	totalRewards := make([]map[common.Hash]uint64, numberOfActiveShards)
	totalRewardForBeacon := map[common.Hash]uint64{}
	totalRewardForIncDAO := map[common.Hash]uint64{}
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
			Logger.log.Infof("\n------------------------------------\nNot enough reward in epoch %v\n------------------------------------\n", err)
		}
		mapPlusMap(rewardForBeacon, &totalRewardForBeacon)
		mapPlusMap(rewardForIncDAO, &totalRewardForIncDAO)
	}
	if len(totalRewardForBeacon) > 0 {
		instRewardForBeacons, err = blockchain.buildInstRewardForBeacons(epoch, totalRewardForBeacon)
		if err != nil {
			return nil, err
		}
	}
	instRewardForShards, err = blockchain.buildInstRewardForShards(epoch, totalRewards)
	if err != nil {
		return nil, err
	}
	if len(totalRewardForIncDAO) > 0 {
		instRewardForIncDAO, err = blockchain.buildInstRewardForIncDAO(epoch, totalRewardForIncDAO)
		if err != nil {
			return nil, err
		}
	}
	resInst = common.AppendSliceString(instRewardForBeacons, instRewardForIncDAO, instRewardForShards)
	return resInst, nil
}

//buildInstRewardForBeacons create reward instruction for beacons
func (blockchain *BlockChain) buildInstRewardForBeacons(epoch uint64, totalReward map[common.Hash]uint64) ([][]string, error) {
	resInst := [][]string{}
	baseRewards := map[common.Hash]uint64{}
	for key, value := range totalReward {
		baseRewards[key] = value / uint64(len(blockchain.BestState.Beacon.BeaconCommittee))
	}
	for _, beaconpublickey := range blockchain.BestState.Beacon.BeaconCommittee {
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

func (blockchain *BlockChain) buildInstRewardForIncDAO(epoch uint64, totalReward map[common.Hash]uint64) ([][]string, error) {
	resInst := [][]string{}
	devRewardInst, err := metadata.BuildInstForIncDAOReward(totalReward, blockchain.config.ChainParams.IncognitoDAOAddress)
	if err != nil {
		Logger.log.Errorf("buildInstRewardForIncDAO error %+v\n Totalreward: %+v, epoch: %+v\n", err, totalReward, epoch)
		return nil, err
	}
	resInst = append(resInst, devRewardInst)
	return resInst, nil
}

func (blockchain *BlockChain) buildInstRewardForShards(epoch uint64, totalRewards []map[common.Hash]uint64) ([][]string, error) {
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

func (blockchain *BlockChain) buildWithDrawTransactionResponse(txRequest *metadata.Transaction, blkProducerPrivateKey *privacy.PrivateKey, shardID byte) (metadata.Transaction, error) {
	if (*txRequest).GetMetadataType() != metadata.WithDrawRewardRequestMeta {
		return nil, errors.New("Can not understand this request!")
	}
	requestDetail := (*txRequest).GetMetadata().(*metadata.WithDrawRewardRequest)
	tempPublicKey := base58.Base58Check{}.Encode(requestDetail.PaymentAddress.Pk, common.Base58Version)
	amount, err := statedb.GetCommitteeReward(blockchain.GetShardRewardStateDB(shardID), tempPublicKey, requestDetail.TokenID)
	if (amount == 0) || (err != nil) {
		return nil, errors.New("Not enough reward")
	}
	responseMeta, err := metadata.NewWithDrawRewardResponse(requestDetail, (*txRequest).Hash())
	if err != nil {
		return nil, err
	}
	return blockchain.InitTxSalaryByCoinID(
		&requestDetail.PaymentAddress,
		amount,
		blkProducerPrivateKey,
		blockchain.BestState.Shard[shardID].GetCopiedTransactionStateDB(),
		blockchain.BestState.Beacon.GetCopiedFeatureStateDB(),
		responseMeta,
		requestDetail.TokenID,
		common.GetShardIDFromLastByte(requestDetail.PaymentAddress.Pk[common.PublicKeySize-1]))
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

func getNoBlkPerYear(blockCreationTimeSeconds uint64) uint64 {
	//31536000 =
	return (365 * 24 * 60 * 60) / blockCreationTimeSeconds
}

func getPercentForIncognitoDAO(blockHeight, blkPerYear uint64) int {
	year := blockHeight / blkPerYear
	if year > (UpperBoundPercentForIncDAO - LowerBoundPercentForIncDAO) {
		return LowerBoundPercentForIncDAO
	} else {
		return UpperBoundPercentForIncDAO - int(year)
	}
}
