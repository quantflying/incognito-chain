package app

import (
	"errors"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
)

func (sca *ShardCoreApp) buildWithDrawTransactionResponse(
	txRequest *metadata.Transaction,
	blockProducerPrivateKey *privacy.PrivateKey,
	transactionStateDB *statedb.StateDB,
	rewardStateDB *statedb.StateDB,
	bridgeStateDB *statedb.StateDB,
) (metadata.Transaction, error) {
	if (*txRequest).GetMetadataType() != metadata.WithDrawRewardRequestMeta {
		return nil, errors.New("Can not understand this request!")
	}
	requestDetail := (*txRequest).GetMetadata().(*metadata.WithDrawRewardRequest)
	tempPublicKey := base58.Base58Check{}.Encode(requestDetail.PaymentAddress.Pk, common.Base58Version)
	amount, err := statedb.GetCommitteeReward(rewardStateDB, tempPublicKey, requestDetail.TokenID)
	if (amount == 0) || (err != nil) {
		return nil, errors.New("Not enough reward")
	}
	responseMeta, err := metadata.NewWithDrawRewardResponse(requestDetail, (*txRequest).Hash())
	if err != nil {
		return nil, err
	}
	return initTxSalaryByCoinID(
		&requestDetail.PaymentAddress,
		amount,
		blockProducerPrivateKey,
		transactionStateDB,
		bridgeStateDB,
		responseMeta,
		requestDetail.TokenID,
		common.GetShardIDFromLastByte(requestDetail.PaymentAddress.Pk[common.PublicKeySize-1]))
}

func (sca ShardCoreApp) buildReturnStakingAmountTx(swapPublicKey string,
	blkProducerPrivateKey *privacy.PrivateKey,
) (metadata.Transaction, error) {
	panic("implement me")
	//addressBytes := blockGenerator.chain.config.UserKeySet.PaymentAddress.Pk
	//shardID := common.GetShardIDFromLastByte(addressBytes[len(addressBytes)-1])
	//publicKey, _ := blockGenerator.chain.config.ConsensusEngine.GetCurrentMiningPublicKey()
	//_, committeeShardID := blockGenerator.chain.BestState.Beacon.GetPubkeyRole(publicKey, 0)
	//
	//fmt.Println("SA: get tx for ", swapPublicKey, GetBestStateShard(committeeShardID).StakingTx, committeeShardID)
	//tx, ok := GetBestStateShard(committeeShardID).StakingTx[swapPublicKey]
	//if !ok {
	//	return nil, NewAppError(GetStakingTransactionError, errors.New("No staking tx in best state"))
	//}
	//var txHash = &common.Hash{}
	//err := (&common.Hash{}).Decode(txHash, tx)
	//if err != nil {
	//	return nil, NewAppError(DecodeHashError, err)
	//}
	//blockHash, index, err := blockGenerator.chain.config.DataBase.GetTransactionIndexById(*txHash)
	//if err != nil {
	//	return nil, NewAppError(GetTransactionFromDatabaseError, err)
	//}
	//shardBlock, _, err := blockGenerator.chain.GetShardBlockByHash(blockHash)
	//if err != nil || shardBlock == nil {
	//	Logger.log.Error("ERROR", err, "NO Transaction in block with hash", blockHash, "and index", index, "contains", shardBlock.Body.Transactions[index])
	//	return nil, NewAppError(FetchShardBlockError, err)
	//}
	//txData := shardBlock.Body.Transactions[index]
	//keyWallet, err := wallet.Base58CheckDeserialize(txData.GetMetadata().(*metadata.StakingMetadata).FunderPaymentAddress)
	//if err != nil {
	//	Logger.log.Error("SA: cannot get payment address", txData.GetMetadata().(*metadata.StakingMetadata), committeeShardID)
	//	return nil, blockchain.NewAppError(blockchain.WalletKeySerializedError, err)
	//}
	//Logger.log.Info("SA: build salary tx", txData.GetMetadata().(*metadata.StakingMetadata).FunderPaymentAddress, committeeShardID)
	//paymentShardID := common.GetShardIDFromLastByte(keyWallet.KeySet.PaymentAddress.Pk[len(keyWallet.KeySet.PaymentAddress.Pk)-1])
	//if paymentShardID != committeeShardID {
	//	return nil, blockchain.NewAppError(blockchain.WrongShardIDError, fmt.Errorf("Staking Payment Address ShardID %+v, Not From Current Shard %+v", paymentShardID, committeeShardID))
	//}
	//returnStakingMeta := metadata.NewReturnStaking(
	//	tx,
	//	keyWallet.KeySet.PaymentAddress,
	//	metadata.ReturnStakingMeta,
	//)
	//returnStakingTx := new(transaction.Tx)
	//err = returnStakingTx.InitTxSalary(
	//	txData.CalculateTxValue(),
	//	&keyWallet.KeySet.PaymentAddress,
	//	blkProducerPrivateKey,
	//	blockGenerator.chain.config.DataBase,
	//	returnStakingMeta,
	//)
	////modify the type of the salary transaction
	//returnStakingTx.Type = common.TxReturnStakingType
	//if err != nil {
	//	return nil, blockchain.NewAppError(blockchain.InitSalaryTransactionError, err)
	//}
	//return returnStakingTx, nil
}
