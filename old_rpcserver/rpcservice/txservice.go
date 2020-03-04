package rpcservice

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/incdb"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/mempool"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/rpcserver/bean"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/incognitochain/incognito-chain/wire"
)

type TxService struct {
	DB           incdb.Database
	BlockChain   *blockchain.BlockChain
	Wallet       *wallet.Wallet
	FeeEstimator map[byte]*mempool.FeeEstimator
	TxMemPool    *mempool.TxPool
}

func (txService TxService) ListSerialNumbers(tokenID common.Hash, shardID byte) (map[string]struct{}, error) {
	transactionStateDB := txService.BlockChain.BestState.Shard[shardID].GetCopiedTransactionStateDB()
	return statedb.ListSerialNumber(transactionStateDB, tokenID, shardID)
}

func (txService TxService) ListSNDerivator(tokenID common.Hash, shardID byte) ([]big.Int, error) {
	transactionStateDB := txService.BlockChain.BestState.Shard[shardID].GetCopiedTransactionStateDB()
	resultInBytes, err := statedb.ListSNDerivator(transactionStateDB, tokenID)
	if err != nil {
		return nil, err
	}
	result := []big.Int{}
	for _, v := range resultInBytes {
		result = append(result, *(new(big.Int).SetBytes(v)))
	}
	return result, nil
}

func (txService TxService) ListCommitments(tokenID common.Hash, shardID byte) (map[string]uint64, error) {
	transactionStateDB := txService.BlockChain.BestState.Shard[shardID].GetCopiedTransactionStateDB()
	return statedb.ListCommitment(transactionStateDB, tokenID, shardID)
}

func (txService TxService) ListCommitmentIndices(tokenID common.Hash, shardID byte) (map[uint64]string, error) {
	transactionStateDB := txService.BlockChain.BestState.Shard[shardID].GetCopiedTransactionStateDB()
	return statedb.ListCommitmentIndices(transactionStateDB, tokenID, shardID)
}

func (txService TxService) HasSerialNumbers(paymentAddressStr string, serialNumbersStr []interface{}, tokenID common.Hash) ([]bool, error) {
	_, shardIDSender, err := GetKeySetFromPaymentAddressParam(paymentAddressStr)
	if err != nil {
		return nil, err
	}
	result := make([]bool, 0)
	for _, item := range serialNumbersStr {
		itemStr, okParam := item.(string)
		if !okParam {
			return nil, fmt.Errorf("Invalid serial number param, %+v", item)
		}
		serialNumber, _, err := base58.Base58Check{}.Decode(itemStr)
		if err != nil {
			return nil, fmt.Errorf("Decode serial number failed, %+v", itemStr)
		}
		transactionStateDB := txService.BlockChain.BestState.Shard[shardIDSender].GetCopiedTransactionStateDB()
		ok, err := statedb.HasSerialNumber(transactionStateDB, tokenID, serialNumber, shardIDSender)
		if ok && err == nil {
			// serial number in db
			result = append(result, true)
		} else {
			// serial number not in db
			result = append(result, false)
		}
	}

	return result, nil
}

func (txService TxService) HasSnDerivators(paymentAddressStr string, snDerivatorStr []interface{}, tokenID common.Hash) ([]bool, error) {
	_, shardIDSender, err := GetKeySetFromPaymentAddressParam(paymentAddressStr)
	if err != nil {
		return nil, err
	}
	result := make([]bool, 0)
	for _, item := range snDerivatorStr {
		itemStr, okParam := item.(string)
		if !okParam {
			return nil, errors.New("Invalid serial number derivator param")
		}
		snderivator, _, err := base58.Base58Check{}.Decode(itemStr)
		if err != nil {
			return nil, errors.New("Invalid serial number derivator param")
		}
		transactionStateDB := txService.BlockChain.BestState.Shard[shardIDSender].GetCopiedTransactionStateDB()
		ok, err := statedb.HasSNDerivator(transactionStateDB, tokenID, common.AddPaddingBigInt(new(big.Int).SetBytes(snderivator), common.BigIntSize))
		if ok && err == nil {
			// SnD in db
			result = append(result, true)
		} else {
			// SnD not in db
			result = append(result, false)
		}
	}
	return result, nil
}

// chooseBestOutCoinsToSpent returns list of unspent coins for spending with amount
func (txService TxService) chooseBestOutCoinsToSpent(outCoins []*privacy.OutputCoin, amount uint64) (resultOutputCoins []*privacy.OutputCoin, remainOutputCoins []*privacy.OutputCoin, totalResultOutputCoinAmount uint64, err error) {
	resultOutputCoins = make([]*privacy.OutputCoin, 0)
	remainOutputCoins = make([]*privacy.OutputCoin, 0)
	totalResultOutputCoinAmount = uint64(0)

	// either take the smallest coins, or a single largest one
	var outCoinOverLimit *privacy.OutputCoin
	outCoinsUnderLimit := make([]*privacy.OutputCoin, 0)
	for _, outCoin := range outCoins {
		if outCoin.CoinDetails.GetValue() < amount {
			outCoinsUnderLimit = append(outCoinsUnderLimit, outCoin)
		} else if outCoinOverLimit == nil {
			outCoinOverLimit = outCoin
		} else if outCoinOverLimit.CoinDetails.GetValue() > outCoin.CoinDetails.GetValue() {
			remainOutputCoins = append(remainOutputCoins, outCoin)
		} else {
			remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
			outCoinOverLimit = outCoin
		}
	}
	sort.Slice(outCoinsUnderLimit, func(i, j int) bool {
		return outCoinsUnderLimit[i].CoinDetails.GetValue() < outCoinsUnderLimit[j].CoinDetails.GetValue()
	})
	for _, outCoin := range outCoinsUnderLimit {
		if totalResultOutputCoinAmount < amount {
			totalResultOutputCoinAmount += outCoin.CoinDetails.GetValue()
			resultOutputCoins = append(resultOutputCoins, outCoin)
		} else {
			remainOutputCoins = append(remainOutputCoins, outCoin)
		}
	}
	if outCoinOverLimit != nil && (outCoinOverLimit.CoinDetails.GetValue() > 2*amount || totalResultOutputCoinAmount < amount) {
		remainOutputCoins = append(remainOutputCoins, resultOutputCoins...)
		resultOutputCoins = []*privacy.OutputCoin{outCoinOverLimit}
		totalResultOutputCoinAmount = outCoinOverLimit.CoinDetails.GetValue()
	} else if outCoinOverLimit != nil {
		remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
	}
	if totalResultOutputCoinAmount < amount {
		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, errors.New("Not enough coin")
	} else {
		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, nil
	}
}

func (txService TxService) filterMemPoolOutcoinsToSpent(outCoins []*privacy.OutputCoin) ([]*privacy.OutputCoin, error) {
	remainOutputCoins := make([]*privacy.OutputCoin, 0)
	for _, outCoin := range outCoins {
		if txService.TxMemPool.ValidateSerialNumberHashH(outCoin.CoinDetails.GetSerialNumber().ToBytesS()) == nil {
			remainOutputCoins = append(remainOutputCoins, outCoin)
		}
	}
	return remainOutputCoins, nil
}

// chooseOutsCoinByKeyset returns list of input coins native token to spent
func (txService TxService) chooseOutsCoinByKeyset(
	paymentInfos []*privacy.PaymentInfo,
	unitFeeNativeToken int64, numBlock uint64, keySet *incognitokey.KeySet, shardIDSender byte,
	hasPrivacy bool,
	metadataParam metadata.Metadata,
	privacyCustomTokenParams *transaction.CustomTokenPrivacyParamTx,
	isGetFeePToken bool,
	unitFeePToken int64,
) ([]*privacy.InputCoin, uint64, *RPCError) {
	// estimate fee according to 8 recent block
	if numBlock == 0 {
		numBlock = 1000
	}
	// calculate total amount to send
	totalAmmount := uint64(0)
	for _, receiver := range paymentInfos {
		totalAmmount += receiver.Amount
	}
	// get list outputcoins tx
	prvCoinID := &common.Hash{}
	prvCoinID.SetBytes(common.PRVCoinID[:])
	outCoins, err := txService.BlockChain.GetListOutputCoinsByKeyset(keySet, shardIDSender, prvCoinID)
	if err != nil {
		return nil, 0, NewRPCError(GetOutputCoinError, err)
	}
	// remove out coin in mem pool
	outCoins, err = txService.filterMemPoolOutcoinsToSpent(outCoins)
	if err != nil {
		return nil, 0, NewRPCError(GetOutputCoinError, err)
	}
	if len(outCoins) == 0 && totalAmmount > 0 {
		return nil, 0, NewRPCError(GetOutputCoinError, errors.New("not enough output coin"))
	}
	// Use Knapsack to get candiate output coin
	candidateOutputCoins, outCoins, candidateOutputCoinAmount, err := txService.chooseBestOutCoinsToSpent(outCoins, totalAmmount)
	if err != nil {
		return nil, 0, NewRPCError(GetOutputCoinError, err)
	}
	// refund out put for sender
	overBalanceAmount := candidateOutputCoinAmount - totalAmmount
	if overBalanceAmount > 0 {
		// add more into output for estimate fee
		paymentInfos = append(paymentInfos, &privacy.PaymentInfo{
			PaymentAddress: keySet.PaymentAddress,
			Amount:         overBalanceAmount,
		})
	}
	// check real fee(nano PRV) per tx
	beaconState, err := txService.BlockChain.BestState.GetClonedBeaconBestState()
	if err != nil {
		return nil, 0, NewRPCError(GetOutputCoinError, err)
	}
	beaconHeight := beaconState.BeaconHeight
	realFee, _, _, err := txService.EstimateFee(unitFeeNativeToken, false, candidateOutputCoins,
		paymentInfos, shardIDSender, numBlock, hasPrivacy,
		metadataParam,
		privacyCustomTokenParams, int64(beaconHeight))
	if err != nil {
		return nil, 0, NewRPCError(GetOutputCoinError, err)
	}
	if totalAmmount == 0 && realFee == 0 {
		if metadataParam != nil {
			metadataType := metadataParam.GetType()
			switch metadataType {
			case metadata.WithDrawRewardRequestMeta:
				{
					return nil, realFee, nil
				}
			}
			return nil, realFee, NewRPCError(GetOutputCoinError, fmt.Errorf("totalAmmount: %+v, realFee: %+v", totalAmmount, realFee))
		}
		if privacyCustomTokenParams != nil {
			// for privacy token
			return nil, 0, nil
		}
	}
	needToPayFee := int64((totalAmmount + realFee) - candidateOutputCoinAmount)
	// if not enough to pay fee
	if needToPayFee > 0 {
		if len(outCoins) > 0 {
			candidateOutputCoinsForFee, _, _, err1 := txService.chooseBestOutCoinsToSpent(outCoins, uint64(needToPayFee))
			if err != nil {
				return nil, 0, NewRPCError(GetOutputCoinError, err1)
			}
			candidateOutputCoins = append(candidateOutputCoins, candidateOutputCoinsForFee...)
		}
	}
	// convert to inputcoins
	inputCoins := transaction.ConvertOutputCoinToInputCoin(candidateOutputCoins)
	return inputCoins, realFee, nil
}

// EstimateFee - estimate fee from tx data and return real full fee, fee per kb and real tx size
// if isGetPTokenFee == true: return fee for ptoken
// if isGetPTokenFee == false: return fee for native token
func (txService TxService) EstimateFee(
	defaultFee int64,
	isGetPTokenFee bool,
	candidateOutputCoins []*privacy.OutputCoin,
	paymentInfos []*privacy.PaymentInfo, shardID byte,
	numBlock uint64, hasPrivacy bool,
	metadata metadata.Metadata,
	privacyCustomTokenParams *transaction.CustomTokenPrivacyParamTx,
	beaconHeight int64) (uint64, uint64, uint64, error) {
	if numBlock == 0 {
		numBlock = 1000
	}
	// check real fee(nano PRV) per tx
	var realFee uint64
	estimateFeeCoinPerKb := uint64(0)
	estimateTxSizeInKb := uint64(0)
	tokenId := &common.Hash{}
	if isGetPTokenFee {
		if privacyCustomTokenParams != nil {
			tokenId, _ = common.Hash{}.NewHashFromStr(privacyCustomTokenParams.PropertyID)
		}
	} else {
		tokenId = nil
	}
	estimateFeeCoinPerKb, err := txService.EstimateFeeWithEstimator(defaultFee, shardID, numBlock, tokenId, beaconHeight)
	if err != nil {
		return 0, 0, 0, err
	}
	if txService.Wallet != nil {
		estimateFeeCoinPerKb += uint64(txService.Wallet.GetConfig().IncrementalFee)
	}
	limitFee := uint64(0)
	if feeEstimator, ok := txService.FeeEstimator[shardID]; ok {
		limitFee = feeEstimator.GetLimitFeeForNativeToken()
	}
	estimateTxSizeInKb = transaction.EstimateTxSize(transaction.NewEstimateTxSizeParam(len(candidateOutputCoins), len(paymentInfos), hasPrivacy, metadata, privacyCustomTokenParams, limitFee))
	realFee = uint64(estimateFeeCoinPerKb) * uint64(estimateTxSizeInKb)
	return realFee, estimateFeeCoinPerKb, estimateTxSizeInKb, nil
}

// EstimateFeeWithEstimator - only estimate fee by estimator and return fee per kb
// if tokenID != nil: return fee per kb for pToken (return error if there is no exchange rate between pToken and native token)
// if tokenID == nil: return fee per kb for native token
func (txService TxService) EstimateFeeWithEstimator(defaultFee int64, shardID byte, numBlock uint64, tokenId *common.Hash, beaconHeight int64) (uint64, error) {
	if defaultFee == 0 {
		return uint64(defaultFee), nil
	}
	unitFee := uint64(0)
	if defaultFee == -1 {
		// estimate fee on the blocks before (in native token or in pToken)
		if _, ok := txService.FeeEstimator[shardID]; ok {
			temp, _ := txService.FeeEstimator[shardID].EstimateFee(numBlock, tokenId)
			unitFee = uint64(temp)
		}
	} else {
		// get default fee (in native token or in ptoken)
		unitFee = uint64(defaultFee)
	}
	// get limit fee for native token
	limitFee := uint64(0)
	if feeEstimator, ok := txService.FeeEstimator[shardID]; ok {
		limitFee = feeEstimator.GetLimitFeeForNativeToken()
	}
	if tokenId == nil {
		// check with limit fee
		if unitFee < limitFee {
			unitFee = limitFee
		}
		return unitFee, nil
	} else {
		// convert limit fee native token to limit fee ptoken
		limitFeePTokenTmp, err := metadata.ConvertNativeTokenToPrivacyToken(limitFee, tokenId, beaconHeight, txService.BlockChain.BestState.Beacon.GetCopiedFeatureStateDB())
		limitFeePToken := uint64(math.Ceil(limitFeePTokenTmp))
		if err != nil {
			return uint64(0), err
		}
		// check with limit fee ptoken
		if unitFee < limitFeePToken {
			unitFee = limitFeePToken
		}
		// add extra fee to make sure tx is confirmed
		// extra fee = unitFee * 10%
		unitFee += uint64(math.Ceil(float64(unitFee) * float64(0.1)))
		return unitFee, nil
	}
}

func (txService TxService) BuildRawTransaction(params *bean.CreateRawTxParam, meta metadata.Metadata) (*transaction.Tx, *RPCError) {
	Logger.log.Infof("Build Raw Transaction Params: \n %+v", params)
	// get output coins to spend and real fee
	inputCoins, realFee, err1 := txService.chooseOutsCoinByKeyset(
		params.PaymentInfos, params.EstimateFeeCoinPerKb, 0,
		params.SenderKeySet, params.ShardIDSender, params.HasPrivacyCoin,
		meta, nil, false, int64(0))
	if err1 != nil {
		return nil, err1
	}
	// init tx
	tx := transaction.Tx{}
	err := tx.Init(
		transaction.NewTxPrivacyInitParams(
			&params.SenderKeySet.PrivateKey,
			params.PaymentInfos,
			inputCoins,
			realFee,
			params.HasPrivacyCoin,
			txService.BlockChain.BestState.Shard[params.ShardIDSender].GetCopiedTransactionStateDB(),
			nil, // use for prv coin -> nil is valid
			meta,
			params.Info,
		))
	if err != nil {
		return nil, NewRPCError(CreateTxDataError, err)
	}
	return &tx, nil
}

func (txService TxService) CreateRawTransaction(params *bean.CreateRawTxParam, meta metadata.Metadata, db incdb.Database) (*common.Hash, []byte, byte, *RPCError) {
	var err error
	tx, err := txService.BuildRawTransaction(params, meta)
	if err.(*RPCError) != nil {
		Logger.log.Critical(err)
		return nil, nil, byte(0), NewRPCError(CreateTxDataError, err)
	}
	txBytes, err := json.Marshal(tx)
	if err != nil {
		// return hex for a new tx
		return nil, nil, byte(0), NewRPCError(CreateTxDataError, err)
	}
	txShardID := common.GetShardIDFromLastByte(tx.GetSenderAddrLastByte())
	return tx.Hash(), txBytes, txShardID, nil
}

func (txService TxService) SendRawTransaction(txB58Check string) (wire.Message, *common.Hash, byte, *RPCError) {
	// Decode base58check data of tx
	rawTxBytes, _, err := base58.Base58Check{}.Decode(txB58Check)
	if err != nil {
		Logger.log.Errorf("Send Raw Transaction Error: %+v", err)
		return nil, nil, byte(0), NewRPCError(SendRawTransactionError, err)
	}
	// Unmarshal from json data to object tx
	var tx transaction.Tx
	err = json.Unmarshal(rawTxBytes, &tx)
	if err != nil {
		Logger.log.Errorf("Send Raw Transaction Error: %+v", err)
		return nil, nil, byte(0), NewRPCError(SendRawTransactionError, err)
	}

	beaconHeigh := int64(-1)
	beaconBestState, err := txService.BlockChain.BestState.GetClonedBeaconBestState()
	if err == nil {
		beaconHeigh = int64(beaconBestState.BeaconHeight)
	} else {
		Logger.log.Errorf("Send Raw Transaction can not get beacon best state with error %+v", err)
	}
	// Try add tx in to mempool of node
	hash, _, err := txService.TxMemPool.MaybeAcceptTransaction(&tx, beaconHeigh)
	if err != nil {
		Logger.log.Errorf("Send Raw Transaction Error, try add tx into mempool of node: %+v", err)
		mempoolErr, ok := err.(*mempool.MempoolTxError)
		if ok {
			switch mempoolErr.Code {
			case mempool.ErrCodeMessage[mempool.RejectInvalidFee].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxFeeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidSize].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxSizeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidTxType].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxTypeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectReplacementTxError].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectReplacementTx, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectDoubleSpendWithBlockchainTx].Code, mempool.ErrCodeMessage[mempool.RejectDoubleSpendWithMempoolTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDoubleSpendTxError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectDuplicateTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDuplicateTxInPoolError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectVersion].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDuplicateTxInPoolError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectSanityTxLocktime].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectSanityTxLocktime, mempoolErr)
				}
			}
		}
		return nil, nil, byte(0), NewRPCError(TxPoolRejectTxError, err)
	}
	Logger.log.Debugf("New transaction hash: %+v \n", *hash)
	// Create tx message for broadcasting
	txMsg, err := wire.MakeEmptyMessage(wire.CmdTx)
	if err != nil {
		Logger.log.Errorf("Send Raw Transaction Error, Create tx message for broadcasting: %+v", err)
		return nil, nil, byte(0), NewRPCError(SendRawTransactionError, err)
	}
	txMsg.(*wire.MessageTx).Transaction = &tx
	return txMsg, hash, tx.PubKeyLastByteSender, nil
}

func (txService TxService) BuildTokenParam(tokenParamsRaw map[string]interface{}, senderKeySet *incognitokey.KeySet, shardIDSender byte) (*transaction.CustomTokenPrivacyParamTx, *RPCError) {
	var privacyTokenParam *transaction.CustomTokenPrivacyParamTx
	var err *RPCError
	isPrivacy, ok := tokenParamsRaw["Privacy"].(bool)
	if !ok {
		return nil, NewRPCError(BuildTokenParamError, fmt.Errorf("Invalid Params %+v", tokenParamsRaw))
	}
	if !isPrivacy {
		// Check normal custom token param
	} else {
		// Check privacy custom token param
		privacyTokenParam, _, _, err = txService.BuildPrivacyCustomTokenParam(tokenParamsRaw, senderKeySet, shardIDSender)
		if err != nil {
			return nil, NewRPCError(BuildTokenParamError, err)
		}
	}
	return privacyTokenParam, nil
}

func (txService TxService) BuildPrivacyCustomTokenParam(tokenParamsRaw map[string]interface{}, senderKeySet *incognitokey.KeySet, shardIDSender byte) (*transaction.CustomTokenPrivacyParamTx, map[common.Hash]transaction.TxCustomTokenPrivacy, map[common.Hash]blockchain.CrossShardTokenPrivacyMetaData, *RPCError) {
	property, ok := tokenParamsRaw["TokenID"].(string)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token ID, Params %+v ", tokenParamsRaw))
	}
	_, ok = tokenParamsRaw["TokenReceivers"]
	if !ok {
		return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New("Token Receiver is invalid"))
	}
	tokenName, ok := tokenParamsRaw["TokenName"].(string)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token Name, Params %+v ", tokenParamsRaw))
	}
	tokenSymbol, ok := tokenParamsRaw["TokenSymbol"].(string)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token Symbol, Params %+v ", tokenParamsRaw))
	}
	tokenTxType, ok := tokenParamsRaw["TokenTxType"].(float64)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token Tx Type, Params %+v ", tokenParamsRaw))
	}
	tokenAmount, ok := tokenParamsRaw["TokenAmount"].(float64)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token Amout, Params %+v ", tokenParamsRaw))
	}
	tokenFee, ok := tokenParamsRaw["TokenFee"].(float64)
	if !ok {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, fmt.Errorf("Invalid Token Fee, Params %+v ", tokenParamsRaw))
	}
	if tokenTxType == transaction.CustomTokenInit {
		tokenFee = 0
	}
	tokenParams := &transaction.CustomTokenPrivacyParamTx{
		PropertyID:     property,
		PropertyName:   tokenName,
		PropertySymbol: tokenSymbol,
		TokenTxType:    int(tokenTxType),
		Amount:         uint64(tokenAmount),
		TokenInput:     nil,
		Fee:            uint64(tokenFee),
	}
	voutsAmount := int64(0)
	var err1 error

	tokenParams.Receiver, voutsAmount, err1 = transaction.CreateCustomTokenPrivacyReceiverArray(tokenParamsRaw["TokenReceivers"])
	if err1 != nil {
		return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, err1)
	}
	voutsAmount += int64(tokenFee)
	// get list custom token
	switch tokenParams.TokenTxType {
	case transaction.CustomTokenTransfer:
		{
			tokenID, err := common.Hash{}.NewHashFromStr(tokenParams.PropertyID)
			if err != nil {
				return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New("Invalid Token ID"))
			}
			isExisted := statedb.PrivacyTokenIDExisted(txService.BlockChain.BestState.Shard[shardIDSender].GetCopiedTransactionStateDB(), *tokenID)
			if !isExisted {
				var isBridgeToken bool
				_, allBridgeTokens, err := txService.BlockChain.GetAllBridgeTokens()
				if err != nil {
					return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New("Invalid Token ID"))
				}
				for _, bridgeToken := range allBridgeTokens {
					if bridgeToken.TokenID.IsEqual(tokenID) {
						isBridgeToken = true
						break
					}
				}
				if !isBridgeToken {
					// totally invalid token
					return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New("Invalid Token ID"))
				}
				//return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, err)
			}
			outputTokens, err := txService.BlockChain.GetListOutputCoinsByKeyset(senderKeySet, shardIDSender, tokenID)
			if err != nil {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, err)
			}
			outputTokens, err = txService.filterMemPoolOutcoinsToSpent(outputTokens)
			if err != nil {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, err)
			}
			candidateOutputTokens, _, _, err := txService.chooseBestOutCoinsToSpent(outputTokens, uint64(voutsAmount))
			if err != nil {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, err)
			}
			intputToken := transaction.ConvertOutputCoinToInputCoin(candidateOutputTokens)
			tokenParams.TokenInput = intputToken
		}
	case transaction.CustomTokenInit:
		{
			if len(tokenParams.Receiver) == 0 {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, errors.New("Init with wrong receiver"))
			}
			if tokenParams.Receiver[0].Amount != tokenParams.Amount { // Init with wrong max amount of custom token
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, errors.New("Init with wrong max amount of property"))
			}
			if tokenParams.PropertyName == "" {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, errors.New("Init with wrong name of property"))
			}
			if tokenParams.PropertySymbol == "" {
				return nil, nil, nil, NewRPCError(BuildPrivacyTokenParamError, errors.New("Init with wrong symbol of property"))
			}
		}
	}
	return tokenParams, nil, nil, nil
}

// BuildRawCustomTokenTransaction ...
func (txService TxService) BuildRawPrivacyCustomTokenTransaction(params interface{}, metaData metadata.Metadata) (*transaction.TxCustomTokenPrivacy, *RPCError) {
	txParam, errParam := bean.NewCreateRawPrivacyTokenTxParam(params)
	if errParam != nil {
		return nil, NewRPCError(RPCInvalidParamsError, errParam)
	}
	tokenParamsRaw := txParam.TokenParamsRaw
	var err error
	tokenParams, err := txService.BuildTokenParam(tokenParamsRaw, txParam.SenderKeySet, txParam.ShardIDSender)

	if err.(*RPCError) != nil {
		return nil, err.(*RPCError)
	}

	if tokenParams == nil {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("can not build token params for request"))
	}
	/******* START choose output native coins(PRV), which is used to create tx *****/
	var inputCoins []*privacy.InputCoin
	realFeePRV := uint64(0)
	inputCoins, realFeePRV, err = txService.chooseOutsCoinByKeyset(txParam.PaymentInfos,
		txParam.EstimateFeeCoinPerKb, 0, txParam.SenderKeySet,
		txParam.ShardIDSender, txParam.HasPrivacyCoin, nil, tokenParams, txParam.IsGetPTokenFee, txParam.UnitPTokenFee)
	if err.(*RPCError) != nil {
		return nil, err.(*RPCError)
	}

	if len(txParam.PaymentInfos) == 0 && realFeePRV == 0 {
		txParam.HasPrivacyCoin = false
	}
	/******* END GET output coins native coins(PRV), which is used to create tx *****/

	tx := &transaction.TxCustomTokenPrivacy{}
	err = tx.Init(
		transaction.NewTxPrivacyTokenInitParams(&txParam.SenderKeySet.PrivateKey,
			txParam.PaymentInfos,
			inputCoins,
			realFeePRV,
			tokenParams,
			txService.BlockChain.BestState.Shard[txParam.ShardIDSender].GetCopiedTransactionStateDB(),
			metaData,
			txParam.HasPrivacyCoin,
			txParam.HasPrivacyToken,
			txParam.ShardIDSender, txParam.Info,
			txService.BlockChain.BestState.Beacon.GetCopiedFeatureStateDB()))
	if err != nil {
		return nil, NewRPCError(CreateTxDataError, err)
	}

	return tx, nil
}

func (txService TxService) GetTransactionHashByReceiver(paymentAddressParam string) (map[byte][]common.Hash, error) {
	var keySet *incognitokey.KeySet

	if paymentAddressParam != "" {
		senderKey, err := wallet.Base58CheckDeserialize(paymentAddressParam)
		if err != nil {
			return nil, errors.New("payment address is invalid")
		}

		keySet = &senderKey.KeySet
	} else {
		return nil, errors.New("payment address is invalid")
	}

	return txService.BlockChain.GetTransactionHashByReceiver(keySet)
}

func (txService TxService) GetTransactionByHash(txHashStr string) (*jsonresult.TransactionDetail, *RPCError) {
	txHash, err := common.Hash{}.NewHashFromStr(txHashStr)
	if err != nil {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("tx hash is invalid"))
	}
	Logger.log.Infof("Get Transaction By Hash %+v", *txHash)

	shardID, blockHash, index, tx, err := txService.BlockChain.GetTransactionByHash(*txHash)
	if err != nil {
		// maybe tx is still in tx mempool -> check mempool
		tx, errM := txService.TxMemPool.GetTx(txHash)
		if errM != nil {
			return nil, NewRPCError(TxNotExistedInMemAndBLockError, errors.New("Tx is not existed in block or mempool"))
		}
		shardIDTemp := common.GetShardIDFromLastByte(tx.GetSenderAddrLastByte())
		result, errM := jsonresult.NewTransactionDetail(tx, nil, 0, 0, shardIDTemp)
		if errM != nil {
			return nil, NewRPCError(UnexpectedError, errM)
		}
		result.IsInMempool = true
		return result, nil
	}
	blockHeight, _, err := rawdbv2.GetIndexOfBlock(txService.DB, blockHash)
	if err != nil {
		return nil, NewRPCError(UnexpectedError, err)
	}
	result, err := jsonresult.NewTransactionDetail(tx, &blockHash, blockHeight, index, shardID)
	if err != nil {
		return nil, NewRPCError(UnexpectedError, err)
	}
	result.IsInBlock = true
	Logger.log.Debugf("handleGetTransactionByHash result: %+v", result)
	return result, nil
}

func (txService TxService) ListPrivacyCustomToken() (map[common.Hash]*statedb.TokenState, error) {
	tokenStates, err := txService.BlockChain.ListAllPrivacyCustomTokenAndPRV()
	if err != nil {
		return tokenStates, err
	}
	delete(tokenStates, common.PRVCoinID)
	return tokenStates, nil
}
func (txService TxService) GetListPrivacyCustomTokenBalance(privateKey string) (jsonresult.ListCustomTokenBalance, *RPCError) {
	result := jsonresult.ListCustomTokenBalance{ListCustomTokenBalance: []jsonresult.CustomTokenBalance{}}
	resultM := make(map[string]jsonresult.CustomTokenBalance)
	account, err := wallet.Base58CheckDeserialize(privateKey)
	if err != nil {
		Logger.log.Debugf("handleGetListPrivacyCustomTokenBalance result: %+v, err: %+v", nil, err)
		return jsonresult.ListCustomTokenBalance{}, NewRPCError(GetListPrivacyCustomTokenBalanceError, err)
	}
	err = account.KeySet.InitFromPrivateKey(&account.KeySet.PrivateKey)
	if err != nil {
		Logger.log.Debugf("handleGetListPrivacyCustomTokenBalance result: %+v, err: %+v", nil, err)
		return jsonresult.ListCustomTokenBalance{}, NewRPCError(GetListPrivacyCustomTokenBalanceError, err)
	}
	result.PaymentAddress = account.Base58CheckSerialize(wallet.PaymentAddressType)
	lastByte := account.KeySet.PaymentAddress.Pk[len(account.KeySet.PaymentAddress.Pk)-1]
	shardIDSender := common.GetShardIDFromLastByte(lastByte)
	tokenStates, err := txService.ListPrivacyCustomToken()
	if err != nil {
		Logger.log.Debugf("handleGetListPrivacyCustomTokenBalance result: %+v, err: %+v", nil, err)
		return jsonresult.ListCustomTokenBalance{}, NewRPCError(GetListPrivacyCustomTokenBalanceError, err)
	}
	tokenIDs := make(map[common.Hash]interface{})
	for tokenID, tokenState := range tokenStates {
		tokenIDs[tokenID] = 0
		item := jsonresult.CustomTokenBalance{}
		item.Name = tokenState.PropertyName()
		item.Symbol = tokenState.PropertySymbol()
		item.TokenID = tokenState.TokenID().String()
		item.TokenImage = common.Render([]byte(item.TokenID))
		balance := uint64(0)
		// get balance for accountName in wallet
		prvCoinID := &common.Hash{}
		err := prvCoinID.SetBytes(common.PRVCoinID[:])
		if err != nil {
			return jsonresult.ListCustomTokenBalance{}, NewRPCError(TokenIsInvalidError, err)
		}
		outcoints, err := txService.BlockChain.GetListOutputCoinsByKeyset(&account.KeySet, shardIDSender, &tokenID)
		if err != nil {
			Logger.log.Debugf("handleGetListPrivacyCustomTokenBalance result: %+v, err: %+v", nil, err)
			return jsonresult.ListCustomTokenBalance{}, NewRPCError(GetListPrivacyCustomTokenBalanceError, err)
		}
		for _, out := range outcoints {
			balance += out.CoinDetails.GetValue()
		}
		item.Amount = balance
		if item.Amount == 0 {
			continue
		}
		item.IsPrivacy = true
		resultM[item.TokenID] = item
	}
	// bridge token
	_, allBridgeTokens, err := txService.BlockChain.GetAllBridgeTokens()
	if err != nil {
		return jsonresult.ListCustomTokenBalance{}, NewRPCError(GetListPrivacyCustomTokenBalanceError, err)
	}
	for _, bridgeToken := range allBridgeTokens {
		if _, ok := tokenIDs[*bridgeToken.TokenID]; ok {
			continue
		}
		item := jsonresult.CustomTokenBalance{}
		item.Name = ""
		item.Symbol = ""
		item.TokenID = bridgeToken.TokenID.String()
		item.TokenImage = common.Render([]byte(item.TokenID))
		tokenID := bridgeToken.TokenID
		balance := uint64(0)
		// get balance for accountName in wallet
		lastByte := account.KeySet.PaymentAddress.Pk[len(account.KeySet.PaymentAddress.Pk)-1]
		shardIDSender := common.GetShardIDFromLastByte(lastByte)
		prvCoinID := &common.Hash{}
		err := prvCoinID.SetBytes(common.PRVCoinID[:])
		if err != nil {
			return jsonresult.ListCustomTokenBalance{}, NewRPCError(TokenIsInvalidError, err)
		}
		outcoints, err := txService.BlockChain.GetListOutputCoinsByKeyset(&account.KeySet, shardIDSender, tokenID)
		if err != nil {
			return jsonresult.ListCustomTokenBalance{}, NewRPCError(UnexpectedError, err)
		}
		for _, out := range outcoints {
			balance += out.CoinDetails.GetValue()
		}
		item.Amount = balance
		if item.Amount == 0 {
			continue
		}
		item.IsPrivacy = true
		item.IsBridgeToken = true
		resultM[item.TokenID] = item
	}
	for _, v := range resultM {
		result.ListCustomTokenBalance = append(result.ListCustomTokenBalance, v)
	}
	return result, nil
}

func (txService TxService) GetBalancePrivacyCustomToken(privateKey string, tokenID string) (uint64, *RPCError) {
	var totalValue uint64 = 0
	account, err := wallet.Base58CheckDeserialize(privateKey)
	if err != nil {
		Logger.log.Debugf("handleGetBalancePrivacyCustomToken result: %+v, err: %+v", nil, err)
		return uint64(0), NewRPCError(UnexpectedError, err)
	}
	err = account.KeySet.InitFromPrivateKey(&account.KeySet.PrivateKey)
	if err != nil {
		Logger.log.Debugf("handleGetBalancePrivacyCustomToken result: %+v, err: %+v", nil, err)
		return uint64(0), NewRPCError(UnexpectedError, err)
	}
	tokenIDs, err := txService.ListPrivacyCustomToken()
	if err != nil {
		Logger.log.Debugf("handleGetListPrivacyCustomTokenBalance result: %+v, err: %+v", nil, err)
		return uint64(0), NewRPCError(UnexpectedError, err)
	}
	for tempTokenID := range tokenIDs {
		if tokenID == tempTokenID.String() {
			lastByte := account.KeySet.PaymentAddress.Pk[len(account.KeySet.PaymentAddress.Pk)-1]
			shardIDSender := common.GetShardIDFromLastByte(lastByte)
			outcoints, err := txService.BlockChain.GetListOutputCoinsByKeyset(&account.KeySet, shardIDSender, &tempTokenID)
			if err != nil {
				Logger.log.Debugf("handleGetBalancePrivacyCustomToken result: %+v, err: %+v", nil, err)
				return uint64(0), NewRPCError(UnexpectedError, err)
			}
			for _, out := range outcoints {
				totalValue += out.CoinDetails.GetValue()
			}
		}
	}
	if totalValue == 0 {
		// bridge token
		allBridgeTokensBytes, err := statedb.GetAllBridgeTokens(txService.BlockChain.BestState.Beacon.GetCopiedFeatureStateDB())
		if err != nil {
			return 0, NewRPCError(UnexpectedError, err)
		}
		if len(allBridgeTokensBytes) > 0 {
			var allBridgeTokens []*rawdbv2.BridgeTokenInfo
			err = json.Unmarshal(allBridgeTokensBytes, &allBridgeTokens)
			if err != nil {
				return 0, NewRPCError(UnexpectedError, err)
			}
			if len(allBridgeTokens) > 0 {
				for _, bridgeToken := range allBridgeTokens {
					tempTokenID := bridgeToken.TokenID
					if tokenID == tempTokenID.String() {
						lastByte := account.KeySet.PaymentAddress.Pk[len(account.KeySet.PaymentAddress.Pk)-1]
						shardIDSender := common.GetShardIDFromLastByte(lastByte)
						outcoints, err := txService.BlockChain.GetListOutputCoinsByKeyset(&account.KeySet, shardIDSender, tempTokenID)
						if err != nil {
							Logger.log.Debugf("handleGetBalancePrivacyCustomToken result: %+v, err: %+v", nil, err)
							return uint64(0), NewRPCError(UnexpectedError, err)
						}
						for _, out := range outcoints {
							totalValue += out.CoinDetails.GetValue()
						}
					}
				}
			}
		}
	}

	return totalValue, nil
}

func (txService TxService) PrivacyCustomTokenDetail(tokenIDStr string) ([]common.Hash, *transaction.TxPrivacyTokenData, error) {
	tokenID, err := common.Hash{}.NewHashFromStr(tokenIDStr)
	if err != nil {
		Logger.log.Debugf("handlePrivacyCustomTokenDetail result: %+v, err: %+v", nil, err)
		return nil, nil, err
	}
	tokenStates, err := txService.ListPrivacyCustomToken()
	if err != nil {
		return nil, nil, err
	}
	tokenData := &transaction.TxPrivacyTokenData{}
	txs := []common.Hash{}
	if token, ok := tokenStates[*tokenID]; ok {
		tokenData.PropertyName = token.PropertyName()
		tokenData.PropertySymbol = token.PropertySymbol()
		txs = append(txs, token.InitTx())
		txs = append(txs, token.Txs()...)
	}
	return txs, tokenData, nil
}

func (txService TxService) RandomCommitments(paymentAddressStr string, outputs []interface{}, tokenID *common.Hash) ([]uint64, []uint64, [][]byte, *RPCError) {
	_, shardIDSender, err := GetKeySetFromPaymentAddressParam(paymentAddressStr)
	if err != nil {
		Logger.log.Debugf("handleRandomCommitments result: %+v, err: %+v", nil, err)
		return nil, nil, nil, NewRPCError(UnexpectedError, err)
	}

	usableOutputCoins := []*privacy.OutputCoin{}
	for _, item := range outputs {
		out, err1 := jsonresult.NewOutcoinFromInterface(item)
		if err1 != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("outputs is invalid", out)))
		}
		valueBNTmp := big.Int{}
		valueBNTmp.SetString(out.Value, 10)

		outputCoin := new(privacy.OutputCoin).Init()
		outputCoin.CoinDetails.SetValue(valueBNTmp.Uint64())

		RandomnessInBytes, _, err := base58.Base58Check{}.Decode(out.Randomness)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("randomness output is invalid", out.Randomness)))
		}
		outputCoin.CoinDetails.SetRandomness(new(privacy.Scalar).FromBytesS(RandomnessInBytes))

		SNDerivatorInBytes, _, err := base58.Base58Check{}.Decode(out.SNDerivator)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("snderivator output is invalid", out.SNDerivator)))
		}
		outputCoin.CoinDetails.SetSNDerivator(new(privacy.Scalar).FromBytesS(SNDerivatorInBytes))

		CoinCommitmentBytes, _, err := base58.Base58Check{}.Decode(out.CoinCommitment)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("coin commitment output is invalid", out.CoinCommitment)))
		}
		CoinCommitment, err := new(privacy.Point).FromBytesS(CoinCommitmentBytes)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("coin commitment output is invalid", CoinCommitmentBytes)))
		}
		outputCoin.CoinDetails.SetCoinCommitment(CoinCommitment)

		PublicKeyBytes, _, err := base58.Base58Check{}.Decode(out.PublicKey)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("public key output is invalid", out.PublicKey)))
		}
		PublicKey, err := new(privacy.Point).FromBytesS(PublicKeyBytes)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("public key output is invalid", PublicKeyBytes)))
		}
		outputCoin.CoinDetails.SetPublicKey(PublicKey)

		InfoBytes, _, err := base58.Base58Check{}.Decode(out.Info)
		if err != nil {
			return nil, nil, nil, NewRPCError(RPCInvalidParamsError, errors.New(fmt.Sprint("Info output is invalid", out.Info)))
		}
		outputCoin.CoinDetails.SetInfo(InfoBytes)

		usableOutputCoins = append(usableOutputCoins, outputCoin)
	}
	usableInputCoins := transaction.ConvertOutputCoinToInputCoin(usableOutputCoins)

	commitmentIndexs, myCommitmentIndexs, commitments := txService.BlockChain.RandomCommitmentsProcess(usableInputCoins, 0, shardIDSender, tokenID)
	return commitmentIndexs, myCommitmentIndexs, commitments, nil
}

func (txService TxService) SendRawPrivacyCustomTokenTransaction(base58CheckData string) (wire.Message, *transaction.TxCustomTokenPrivacy, error) {
	rawTxBytes, _, err := base58.Base58Check{}.Decode(base58CheckData)
	if err != nil {
		Logger.log.Debugf("handleSendRawPrivacyCustomTokenTransaction result: %+v, err: %+v", nil, err)
		return nil, nil, err
	}

	tx := transaction.TxCustomTokenPrivacy{}
	err = json.Unmarshal(rawTxBytes, &tx)
	if err != nil {
		Logger.log.Debugf("handleSendRawPrivacyCustomTokenTransaction result: %+v, err: %+v", nil, err)
		return nil, nil, err
	}

	beaconHeigh := int64(-1)
	beaconBestState, err := txService.BlockChain.BestState.GetClonedBeaconBestState()
	if err == nil {
		beaconHeigh = int64(beaconBestState.BeaconHeight)
	}
	hash, _, err := txService.TxMemPool.MaybeAcceptTransaction(&tx, beaconHeigh)
	//httpServer.config.NetSync.HandleCacheTxHash(*tx.Hash())
	if err != nil {
		Logger.log.Debugf("handleSendRawPrivacyCustomTokenTransaction result: %+v, err: %+v", nil, err)
		return nil, nil, err
	}

	Logger.log.Debugf("there is hash of transaction: %s\n", hash.String())

	txMsg, err := wire.MakeEmptyMessage(wire.CmdPrivacyCustomToken)
	if err != nil {
		Logger.log.Debugf("handleSendRawPrivacyCustomTokenTransaction result: %+v, err: %+v", nil, err)
		return nil, nil, err
	}

	txMsg.(*wire.MessageTxPrivacyToken).Transaction = &tx

	return txMsg, &tx, nil
}

func (txService TxService) BuildRawDefragmentAccountTransaction(params interface{}, meta metadata.Metadata, db incdb.Database) (*transaction.Tx, *RPCError) {
	arrayParams := common.InterfaceSlice(params)
	if len(arrayParams) < 4 {
		return nil, NewRPCError(RPCInvalidParamsError, nil)
	}

	senderKeyParam, ok := arrayParams[0].(string)
	if !ok {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("senderKeyParam is invalid"))
	}

	maxValTemp, ok := arrayParams[1].(float64)
	if !ok {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("maxVal is invalid"))
	}
	maxVal := uint64(maxValTemp)

	estimateFeeCoinPerKbtemp, ok := arrayParams[2].(float64)
	if !ok {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("estimateFeeCoinPerKb is invalid"))
	}
	estimateFeeCoinPerKb := int64(estimateFeeCoinPerKbtemp)

	// param #4: hasPrivacyCoin flag: 1 or -1
	hasPrivacyCoinParam := arrayParams[3].(float64)
	hasPrivacyCoin := int(hasPrivacyCoinParam) > 0
	/********* END Fetch all component to *******/

	// param #1: private key of sender
	senderKeySet, shardIDSender, err := GetKeySetFromPrivateKeyParams(senderKeyParam)
	if err != nil {
		return nil, NewRPCError(InvalidSenderPrivateKeyError, err)
	}

	prvCoinID := &common.Hash{}
	err1 := prvCoinID.SetBytes(common.PRVCoinID[:])
	if err1 != nil {
		return nil, NewRPCError(TokenIsInvalidError, err1)
	}
	outCoins, err := txService.BlockChain.GetListOutputCoinsByKeyset(senderKeySet, shardIDSender, prvCoinID)
	if err != nil {
		return nil, NewRPCError(GetOutputCoinError, err)
	}
	// remove out coin in mem pool
	outCoins, err = txService.filterMemPoolOutcoinsToSpent(outCoins)
	if err != nil {
		return nil, NewRPCError(GetOutputCoinError, err)
	}
	outCoins, amount := txService.calculateOutputCoinsByMinValue(outCoins, maxVal)
	if len(outCoins) == 0 {
		return nil, NewRPCError(GetOutputCoinError, nil)
	}
	paymentInfo := &privacy.PaymentInfo{
		Amount:         uint64(amount),
		PaymentAddress: senderKeySet.PaymentAddress,
		Message:        []byte{},
	}
	paymentInfos := []*privacy.PaymentInfo{paymentInfo}
	// check real fee(nano PRV) per tx
	isGetPTokenFee := false
	beaconState, err := txService.BlockChain.BestState.GetClonedBeaconBestState()
	if err != nil {
		return nil, NewRPCError(GetOutputCoinError, err)
	}
	beaconHeight := beaconState.BeaconHeight
	realFee, _, _, _ := txService.EstimateFee(
		estimateFeeCoinPerKb, isGetPTokenFee, outCoins, paymentInfos, shardIDSender, 8, hasPrivacyCoin, nil, nil, int64(beaconHeight))
	if len(outCoins) == 0 {
		realFee = 0
	}
	if uint64(amount) < realFee {
		return nil, NewRPCError(GetOutputCoinError, err)
	}
	paymentInfo.Amount = uint64(amount) - realFee
	inputCoins := transaction.ConvertOutputCoinToInputCoin(outCoins)
	/******* END GET output native coins(PRV), which is used to create tx *****/
	// START create tx
	// missing flag for privacy
	// false by default
	tx := transaction.Tx{}
	err = tx.Init(
		transaction.NewTxPrivacyInitParams(&senderKeySet.PrivateKey,
			paymentInfos,
			inputCoins,
			realFee,
			hasPrivacyCoin,
			txService.BlockChain.BestState.Shard[shardIDSender].GetCopiedTransactionStateDB(),
			nil, // use for prv coin -> nil is valid
			meta, nil))
	// END create tx

	if err != nil {
		return nil, NewRPCError(CreateTxDataError, err)
	}

	return &tx, nil
}

//calculateOutputCoinsByMinValue
func (txService TxService) calculateOutputCoinsByMinValue(outCoins []*privacy.OutputCoin, maxVal uint64) ([]*privacy.OutputCoin, uint64) {
	outCoinsTmp := make([]*privacy.OutputCoin, 0)
	amount := uint64(0)
	for _, outCoin := range outCoins {
		if outCoin.CoinDetails.GetValue() <= maxVal {
			outCoinsTmp = append(outCoinsTmp, outCoin)
			amount += outCoin.CoinDetails.GetValue()
		}
	}
	return outCoinsTmp, amount
}

/*func (txService TxService) SendRawTxWithMetadata(txBase58CheckData string) (wire.Message, *common.Hash, byte, *RPCError) {
	// Decode base58check data of tx
	rawTxBytes, _, err := base58.Base58Check{}.Decode(txBase58CheckData)
	if err != nil {
		Logger.log.Errorf("txService.SendRawTxWithMetadata fail with err: %+v", err)
		return nil, nil, byte(0), NewRPCError(Base58ChedkDataOfTxInvalid, err)
	}

	// Unmarshal from json data to object tx
	tx := transaction.Tx{}
	err = json.Unmarshal(rawTxBytes, &tx)
	if err != nil {
		Logger.log.Errorf("txService.SendRawTxWithMetadata fail with err: %+v", err)
		return nil, nil, byte(0), NewRPCError(JsonDataOfTxInvalid, err)
	}

	beaconHeigh := int64(-1)
	beaconBestState, err := txService.BlockChain.BestState.GetClonedBeaconBestState()
	if err == nil {
		beaconHeigh = int64(beaconBestState.BeaconHeight)
	} else {
		Logger.log.Errorf("txService.SendRawTransaction can not get beacon best state with error %+v", err)
	}

	// Try add tx in to mempool of node
	hash, _, err := txService.TxMemPool.MaybeAcceptTransaction(&tx, beaconHeigh)
	if err != nil {
		Logger.log.Errorf("txService.SendRawTxWithMetadata Try add tx into mempool of node with err: %+v", err)
		mempoolErr, ok := err.(*mempool.MempoolTxError)
		if ok {
			switch mempoolErr.Code {
			case mempool.ErrCodeMessage[mempool.RejectInvalidFee].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxFeeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidSize].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxSizeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidTxType].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxTypeError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectInvalidTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectInvalidTxError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectDoubleSpendWithBlockchainTx].Code, mempool.ErrCodeMessage[mempool.RejectDoubleSpendWithMempoolTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDoubleSpendTxError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectDuplicateTx].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDuplicateTxInPoolError, mempoolErr)
				}
			case mempool.ErrCodeMessage[mempool.RejectVersion].Code:
				{
					return nil, nil, byte(0), NewRPCError(RejectDuplicateTxInPoolError, mempoolErr)
				}
			}
		}
		return nil, nil, byte(0), NewRPCError(TxPoolRejectTxError, err)
	}

	Logger.log.Debugf("there is hash of transaction: %s\n", hash.String())

	// Create tx message for broadcasting
	txMsg, err := wire.MakeEmptyMessage(wire.CmdTx)
	if err != nil {
		Logger.log.Errorf("txService.SendRawTxWithMetadata Create tx message for broadcasting with err: %+v", err)
		return nil, nil, byte(0), NewRPCError(SendTxDataError, err)
	}

	txMsg.(*wire.MessageTx).Transaction = &tx

	return txMsg, hash, tx.PubKeyLastByteSender, nil
}*/

// GetTransactionByReceiver - from keyset of receiver, we can get list tx hash which be sent to receiver
// if this keyset contain payment-addr, we can detect tx hash
// if this keyset contain viewing key, we can detect amount in tx, but can not know output in tx is spent
// because this is monitoring output to get received tx -> can not know this is a returned amount tx
func (txService TxService) GetTransactionByReceiver(keySet incognitokey.KeySet) (*jsonresult.ListReceivedTransaction, *RPCError) {
	if len(keySet.PaymentAddress.Pk) == 0 {
		return nil, NewRPCError(RPCInvalidParamsError, errors.New("Missing payment address"))
	}
	listTxsHash, err := txService.BlockChain.GetTransactionHashByReceiver(&keySet)
	if err != nil {
		return nil, NewRPCError(UnexpectedError, errors.New("Can not find any tx"))
	}

	result := jsonresult.ListReceivedTransaction{
		ReceivedTransactions: []jsonresult.ReceivedTransaction{},
	}
	for shardID, txHashs := range listTxsHash {
		for _, txHash := range txHashs {
			item := jsonresult.ReceivedTransaction{
				FromShardID:     shardID,
				ReceivedAmounts: make(map[common.Hash]jsonresult.ReceivedInfo),
			}
			if len(keySet.ReadonlyKey.Rk) != 0 {
				_, blockHash, _, txDetail, _ := txService.BlockChain.GetTransactionByHash(txHash)
				item.LockTime = time.Unix(txDetail.GetLockTime(), 0).Format(common.DateOutputFormat)
				item.Info = base58.Base58Check{}.Encode(txDetail.GetInfo(), common.ZeroByte)
				item.BlockHash = blockHash.String()
				item.Hash = txDetail.Hash().String()

				txType := txDetail.GetType()
				item.Type = txType
				switch item.Type {
				case common.TxNormalType, common.TxRewardType, common.TxReturnStakingType:
					{
						normalTx := txDetail.(*transaction.Tx)
						item.Version = normalTx.Version
						item.IsPrivacy = normalTx.IsPrivacy()
						item.Fee = normalTx.Fee

						proof := normalTx.GetProof()
						if proof != nil {
							outputs := proof.GetOutputCoins()
							for _, output := range outputs {
								if bytes.Equal(output.CoinDetails.GetPublicKey().ToBytesS(), keySet.PaymentAddress.Pk) {
									temp := &privacy.OutputCoin{
										CoinDetails:          output.CoinDetails,
										CoinDetailsEncrypted: output.CoinDetailsEncrypted,
									}
									if temp.CoinDetailsEncrypted != nil && !temp.CoinDetailsEncrypted.IsNil() {
										// try to decrypt to get more data
										err := temp.Decrypt(keySet.ReadonlyKey)
										if err != nil {
											Logger.log.Error(err)
											continue
										}
									}
									info := jsonresult.ReceivedInfo{
										CoinDetails: jsonresult.ReceivedCoin{
											Info:      base58.Base58Check{}.Encode(temp.CoinDetails.GetInfo(), common.ZeroByte),
											PublicKey: base58.Base58Check{}.Encode(temp.CoinDetails.GetPublicKey().ToBytesS(), common.ZeroByte),
											Value:     temp.CoinDetails.GetValue(),
										},
									}
									if temp.CoinDetailsEncrypted != nil {
										info.CoinDetailsEncrypted = base58.Base58Check{}.Encode(temp.CoinDetailsEncrypted.Bytes(), common.ZeroByte)
									}
									item.ReceivedAmounts[common.PRVCoinID] = info
								}
							}
						}
					}
				case common.TxCustomTokenPrivacyType:
					{
						privacyTokenTx := txDetail.(*transaction.TxCustomTokenPrivacy)
						item.Version = privacyTokenTx.Version
						item.IsPrivacy = privacyTokenTx.IsPrivacy()
						item.PrivacyCustomTokenIsPrivacy = privacyTokenTx.TxPrivacyTokenData.TxNormal.IsPrivacy()
						item.Fee = privacyTokenTx.Fee
						item.PrivacyCustomTokenFee = privacyTokenTx.TxPrivacyTokenData.TxNormal.Fee
						item.PrivacyCustomTokenID = privacyTokenTx.TxPrivacyTokenData.PropertyID.String()
						item.PrivacyCustomTokenName = privacyTokenTx.TxPrivacyTokenData.PropertyName
						item.PrivacyCustomTokenSymbol = privacyTokenTx.TxPrivacyTokenData.PropertySymbol

						// prv proof
						proof := privacyTokenTx.GetProof()
						if proof != nil {
							outputs := proof.GetOutputCoins()
							for _, output := range outputs {
								if bytes.Equal(output.CoinDetails.GetPublicKey().ToBytesS(), keySet.PaymentAddress.Pk) {
									temp := &privacy.OutputCoin{
										CoinDetails:          output.CoinDetails,
										CoinDetailsEncrypted: output.CoinDetailsEncrypted,
									}
									if temp.CoinDetailsEncrypted != nil && !temp.CoinDetailsEncrypted.IsNil() {
										// try to decrypt to get more data
										err := temp.Decrypt(keySet.ReadonlyKey)
										if err != nil {
											Logger.log.Error(err)
											continue
										}
									}
									info := jsonresult.ReceivedInfo{
										CoinDetails: jsonresult.ReceivedCoin{
											Info:      base58.Base58Check{}.Encode(temp.CoinDetails.GetInfo(), common.ZeroByte),
											PublicKey: base58.Base58Check{}.Encode(temp.CoinDetails.GetPublicKey().ToBytesS(), common.ZeroByte),
											Value:     temp.CoinDetails.GetValue(),
										},
									}
									if temp.CoinDetailsEncrypted != nil {
										info.CoinDetailsEncrypted = base58.Base58Check{}.Encode(temp.CoinDetailsEncrypted.Bytes(), common.ZeroByte)
									}
									item.ReceivedAmounts[common.PRVCoinID] = info
								}
							}
						}

						// token proof
						proof = privacyTokenTx.TxPrivacyTokenData.TxNormal.GetProof()
						if proof != nil {
							outputs := proof.GetOutputCoins()
							for _, output := range outputs {
								if bytes.Equal(output.CoinDetails.GetPublicKey().ToBytesS(), keySet.PaymentAddress.Pk) {
									temp := &privacy.OutputCoin{
										CoinDetails:          output.CoinDetails,
										CoinDetailsEncrypted: output.CoinDetailsEncrypted,
									}
									if temp.CoinDetailsEncrypted != nil && !temp.CoinDetailsEncrypted.IsNil() {
										// try to decrypt to get more data
										err := temp.Decrypt(keySet.ReadonlyKey)
										if err != nil {
											Logger.log.Error(err)
											continue
										}
									}
									info := jsonresult.ReceivedInfo{
										CoinDetails: jsonresult.ReceivedCoin{
											Info:      base58.Base58Check{}.Encode(temp.CoinDetails.GetInfo(), common.ZeroByte),
											PublicKey: base58.Base58Check{}.Encode(temp.CoinDetails.GetPublicKey().ToBytesS(), common.ZeroByte),
											Value:     temp.CoinDetails.GetValue(),
										},
									}
									if temp.CoinDetailsEncrypted != nil {
										info.CoinDetailsEncrypted = base58.Base58Check{}.Encode(temp.CoinDetailsEncrypted.Bytes(), common.ZeroByte)
									}
									item.ReceivedAmounts[privacyTokenTx.TxPrivacyTokenData.PropertyID] = info
								}
							}
						}
					}
				}
			}
			result.ReceivedTransactions = append(result.ReceivedTransactions, item)
			sort.Slice(result.ReceivedTransactions, func(i, j int) bool {
				return result.ReceivedTransactions[i].LockTime > result.ReceivedTransactions[j].LockTime
			})
		}
	}
	return &result, nil
}
