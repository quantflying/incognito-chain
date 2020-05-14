package blockchain

import (
	"encoding/json"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
)

// buildPortalRefundCustodianDepositTx builds refund tx for custodian deposit tx with status "refund"
// mints PRV to return to custodian
func (curView *ShardBestState) buildPortalRefundCustodianDepositTx(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Info("[Portal refund custodian deposit] Starting...")
	contentBytes := []byte(contentStr)
	var refundDeposit metadata.PortalCustodianDepositContent
	err := json.Unmarshal(contentBytes, &refundDeposit)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal custodian deposit content: %+v", err)
		return nil, nil
	}
	if refundDeposit.ShardID != shardID {
		return nil, nil
	}

	meta := metadata.NewPortalCustodianDepositResponse(
		"refund",
		refundDeposit.TxReqID,
		refundDeposit.IncogAddressStr,
		metadata.PortalCustodianDepositResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(refundDeposit.IncogAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing custodian address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		refundDeposit.DepositedAmount,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing refund contribution (normal) tx: %+v", err)
		return nil, nil
	}
	//modify the type of the salary transaction
	// resTx.Type = common.TxBlockProducerCreatedType
	return resTx, nil
}

func (curView *ShardBestState) buildPortalLiquidationCustodianDepositReject(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Info("[buildPortalLiquidationCustodianDepositReject] Starting...")
	contentBytes := []byte(contentStr)
	var refundDeposit metadata.PortalLiquidationCustodianDepositContent
	err := json.Unmarshal(contentBytes, &refundDeposit)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshaling portal liquidation custodian deposit content: %+v", err)
		return nil, nil
	}
	if refundDeposit.ShardID != shardID {
		return nil, nil
	}

	meta := metadata.NewPortalLiquidationCustodianDepositResponse(
		common.PortalLiquidationCustodianDepositRejectedChainStatus,
		refundDeposit.TxReqID,
		refundDeposit.IncogAddressStr,
		refundDeposit.DepositedAmount,
		metadata.PortalLiquidationCustodianDepositResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(refundDeposit.IncogAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while deserializing custodian liquidation address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		refundDeposit.DepositedAmount,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while initializing refund contribution (normal) tx: %+v", err)
		return nil, nil
	}
	//modify the type of the salary transaction
	// resTx.Type = common.TxBlockProducerCreatedType
	return resTx, nil
}

// buildPortalAcceptedRequestPTokensTx builds response tx for user request ptoken tx with status "accepted"
// mints ptoken to return to user
func (curView *ShardBestState) buildPortalAcceptedRequestPTokensTx(
	beaconState *BeaconBestState,
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Errorf("[Shard buildPortalAcceptedRequestPTokensTx] Starting...")
	contentBytes := []byte(contentStr)
	var acceptedReqPToken metadata.PortalRequestPTokensContent
	err := json.Unmarshal(contentBytes, &acceptedReqPToken)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal custodian deposit content: %+v", err)
		return nil, nil
	}
	if acceptedReqPToken.ShardID != shardID {
		Logger.log.Errorf("ERROR: ShardID unexpected expect %v, but got %+v", shardID, acceptedReqPToken.ShardID)
		return nil, nil
	}

	meta := metadata.NewPortalRequestPTokensResponse(
		"accepted",
		acceptedReqPToken.TxReqID,
		acceptedReqPToken.IncogAddressStr,
		acceptedReqPToken.PortingAmount,
		acceptedReqPToken.TokenID,
		metadata.PortalUserRequestPTokenResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(acceptedReqPToken.IncogAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing custodian address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress
	receiveAmt := acceptedReqPToken.PortingAmount
	tokenID, _ := new(common.Hash).NewHashFromStr(acceptedReqPToken.TokenID)

	// in case the returned currency is privacy custom token
	receiver := &privacy.PaymentInfo{
		Amount:         receiveAmt,
		PaymentAddress: receiverAddr,
	}
	var propertyID [common.HashSize]byte
	copy(propertyID[:], tokenID[:])
	propID := common.Hash(propertyID)
	tokenParams := &transaction.CustomTokenPrivacyParamTx{
		PropertyID: propID.String(),
		// PropertyName:   issuingAcceptedInst.IncTokenName,
		// PropertySymbol: issuingAcceptedInst.IncTokenName,
		Amount:      receiveAmt,
		TokenTxType: transaction.CustomTokenInit,
		Receiver:    []*privacy.PaymentInfo{receiver},
		TokenInput:  []*privacy.InputCoin{},
		Mintable:    true,
	}
	resTx := &transaction.TxCustomTokenPrivacy{}
	txStateDB := curView.GetCopiedTransactionStateDB()
	featureStateDB := beaconState.GetBeaconFeatureStateDB()
	initErr := resTx.Init(
		transaction.NewTxPrivacyTokenInitParams(
			producerPrivateKey,
			[]*privacy.PaymentInfo{},
			nil,
			0,
			tokenParams,
			txStateDB,
			meta,
			false,
			false,
			shardID,
			nil,
			featureStateDB,
		),
	)
	if initErr != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing request ptoken response tx: %+v", initErr)
		return nil, initErr
	}
	return resTx, nil
}

func (curView *ShardBestState) buildPortalCustodianWithdrawRequest(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Infof("[Shard buildPortalCustodianWithdrawRequest] Starting...")
	contentBytes := []byte(contentStr)
	var custodianWithdrawRequest metadata.PortalCustodianWithdrawRequestContent
	err := json.Unmarshal(contentBytes, &custodianWithdrawRequest)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshaling portal custodian withdraw request content: %+v", err)
		return nil, nil
	}
	if custodianWithdrawRequest.ShardID != shardID {
		Logger.log.Errorf("ERROR: ShardID unexpected expect %v, but got %+v", shardID, custodianWithdrawRequest.ShardID)
		return nil, nil
	}

	meta := metadata.NewPortalCustodianWithdrawResponse(
		common.PortalCustodianWithdrawRequestAcceptedStatus,
		custodianWithdrawRequest.TxReqID,
		custodianWithdrawRequest.PaymentAddress,
		custodianWithdrawRequest.Amount,
		metadata.PortalCustodianWithdrawResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(custodianWithdrawRequest.PaymentAddress)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while deserializing custodian address string: %+v", err)
		return nil, nil
	}

	receiverAddr := keyWallet.KeySet.PaymentAddress
	receiveAmt := custodianWithdrawRequest.Amount

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		receiveAmt,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing custodian withdraw  (normal) tx: %+v", err)
		return nil, nil
	}

	return resTx, nil
}

func (curView *ShardBestState) buildPortalRedeemLiquidateExchangeRatesRequestTx(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Errorf("[Shard buildPortalRedeemLiquidateExchangeRatesRequestTx] Starting...")
	contentBytes := []byte(contentStr)
	var redeemReqContent metadata.PortalRedeemLiquidateExchangeRatesContent
	err := json.Unmarshal(contentBytes, &redeemReqContent)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshaling portal redeem liquidate exchange rates content: %+v", err)
		return nil, nil
	}
	if redeemReqContent.ShardID != shardID {
		Logger.log.Errorf("ERROR: ShardID unexpected expect %v, but got %+v", shardID, redeemReqContent.ShardID)
		return nil, nil
	}

	meta := metadata.NewPortalRedeemLiquidateExchangeRatesResponse(
		common.PortalRedeemLiquidateExchangeRatesSuccessChainStatus,
		redeemReqContent.TxReqID,
		redeemReqContent.RedeemerIncAddressStr,
		redeemReqContent.RedeemAmount,
		redeemReqContent.TotalPTokenReceived,
		redeemReqContent.TokenID,
		metadata.PortalRedeemLiquidateExchangeRatesResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(redeemReqContent.RedeemerIncAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while deserializing custodian address string: %+v", err)
		return nil, nil
	}

	receiverAddr := keyWallet.KeySet.PaymentAddress
	receiveAmt := redeemReqContent.TotalPTokenReceived

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		receiveAmt,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing custodian withdraw  (normal) tx: %+v", err)
		return nil, nil
	}

	return resTx, nil
}

// buildPortalRejectedRedeemRequestTx builds response tx for user request redeem tx with status "rejected"
// mints ptoken to return to user (ptoken that user burned)
func (curView *ShardBestState) buildPortalRejectedRedeemRequestTx(
	beaconState *BeaconBestState,
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Errorf("[Shard buildPortalRejectedRedeemRequestTx] Starting...")
	contentBytes := []byte(contentStr)
	var redeemReqContent metadata.PortalRedeemRequestContent
	err := json.Unmarshal(contentBytes, &redeemReqContent)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal redeem request content: %+v", err)
		return nil, nil
	}
	if redeemReqContent.ShardID != shardID {
		Logger.log.Errorf("ERROR: unexpected ShardID, expect %v, but got %+v", shardID, redeemReqContent.ShardID)
		return nil, nil
	}

	meta := metadata.NewPortalRedeemRequestResponse(
		"rejected",
		redeemReqContent.TxReqID,
		redeemReqContent.RedeemerIncAddressStr,
		redeemReqContent.RedeemAmount,
		redeemReqContent.TokenID,
		metadata.PortalRedeemRequestResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(redeemReqContent.RedeemerIncAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing requester address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress
	receiveAmt := redeemReqContent.RedeemAmount
	tokenID, _ := new(common.Hash).NewHashFromStr(redeemReqContent.TokenID)

	// in case the returned currency is privacy custom token
	refundedPTokenPaymentInfo := &privacy.PaymentInfo{
		Amount:         receiveAmt,
		PaymentAddress: receiverAddr,
	}
	var propertyID [common.HashSize]byte
	copy(propertyID[:], tokenID[:])
	propID := common.Hash(propertyID)
	tokenParams := &transaction.CustomTokenPrivacyParamTx{
		PropertyID:  propID.String(),
		Amount:      receiveAmt,
		TokenTxType: transaction.CustomTokenInit,
		Receiver:    []*privacy.PaymentInfo{refundedPTokenPaymentInfo},
		TokenInput:  []*privacy.InputCoin{},
		Mintable:    true,
	}
	resTx := &transaction.TxCustomTokenPrivacy{}
	txStateDB := curView.GetCopiedTransactionStateDB()
	featureStateDB := beaconState.GetBeaconFeatureStateDB()
	initErr := resTx.Init(
		transaction.NewTxPrivacyTokenInitParams(
			producerPrivateKey,
			[]*privacy.PaymentInfo{},
			nil,
			0,
			tokenParams,
			txStateDB,
			meta,
			false,
			false,
			shardID,
			nil,
			featureStateDB,
		),
	)
	if initErr != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing redeem request response tx: %+v", initErr)
		return nil, initErr
	}

	Logger.log.Info("[Shard buildPortalRejectedRedeemRequestTx] Finished...")
	return resTx, nil
}

// buildPortalRefundCustodianDepositTx builds refund tx for custodian deposit tx with status "refund"
// mints PRV to return to custodian
func (curView *ShardBestState) buildPortalLiquidateCustodianResponseTx(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Info("[Portal liquidate custodian response] Starting...")
	contentBytes := []byte(contentStr)
	var liqCustodian metadata.PortalLiquidateCustodianContent
	err := json.Unmarshal(contentBytes, &liqCustodian)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal liquidation custodian content: %+v", err)
		return nil, nil
	}
	if liqCustodian.ShardID != shardID {
		return nil, nil
	}

	meta := metadata.NewPortalLiquidateCustodianResponse(
		liqCustodian.UniqueRedeemID,
		liqCustodian.LiquidatedCollateralAmount,
		liqCustodian.RedeemerIncAddressStr,
		liqCustodian.CustodianIncAddressStr,
		metadata.PortalLiquidateCustodianResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(liqCustodian.RedeemerIncAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing redeemer address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		liqCustodian.LiquidatedCollateralAmount,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing refund contribution (normal) tx: %+v", err)
		return nil, nil
	}
	return resTx, nil
}

// buildPortalAcceptedWithdrawRewardTx builds withdraw portal rewards response tx
// mints rewards in PRV for sending to custodian
func (curView *ShardBestState) buildPortalAcceptedWithdrawRewardTx(
	baeconState *BeaconBestState,
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Info("[buildPortalAcceptedWithdrawRewardTx] Starting...")
	contentBytes := []byte(contentStr)
	var withdrawRewardContent metadata.PortalRequestWithdrawRewardContent
	err := json.Unmarshal(contentBytes, &withdrawRewardContent)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal withdraw reward content: %+v", err)
		return nil, nil
	}
	if withdrawRewardContent.ShardID != shardID {
		return nil, nil
	}

	meta := metadata.NewPortalWithdrawRewardResponse(
		withdrawRewardContent.TxReqID,
		withdrawRewardContent.CustodianAddressStr,
		withdrawRewardContent.TokenID,
		withdrawRewardContent.RewardAmount,
		metadata.PortalRequestWithdrawRewardResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(withdrawRewardContent.CustodianAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing custodian address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress

	// the returned currency is PRV
	if withdrawRewardContent.TokenID.String() == common.PRVIDStr {
		resTx := new(transaction.Tx)
		err = resTx.InitTxSalary(
			withdrawRewardContent.RewardAmount,
			&receiverAddr,
			producerPrivateKey,
			curView.GetCopiedTransactionStateDB(),
			meta,
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while initializing withdraw portal reward tx: %+v", err)
			return nil, nil
		}
		return resTx, nil
	} else {
		// in case the returned currency is privacy custom token
		receiver := &privacy.PaymentInfo{
			Amount:         withdrawRewardContent.RewardAmount,
			PaymentAddress: receiverAddr,
		}
		var propertyID [common.HashSize]byte
		copy(propertyID[:], withdrawRewardContent.TokenID[:])
		propID := common.Hash(propertyID)
		tokenParams := &transaction.CustomTokenPrivacyParamTx{
			PropertyID: propID.String(),
			// PropertyName:   issuingAcceptedInst.IncTokenName,
			// PropertySymbol: issuingAcceptedInst.IncTokenName,
			Amount:      withdrawRewardContent.RewardAmount,
			TokenTxType: transaction.CustomTokenInit,
			Receiver:    []*privacy.PaymentInfo{receiver},
			TokenInput:  []*privacy.InputCoin{},
			Mintable:    true,
		}
		resTx := &transaction.TxCustomTokenPrivacy{}
		txStateDB := curView.GetCopiedTransactionStateDB()
		featureStateDB := baeconState.GetBeaconFeatureStateDB()
		err = resTx.Init(
			transaction.NewTxPrivacyTokenInitParams(
				producerPrivateKey,
				[]*privacy.PaymentInfo{},
				nil,
				0,
				tokenParams,
				txStateDB,
				meta,
				false,
				false,
				shardID,
				nil,
				featureStateDB,
			),
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while initializing withdraw portal reward tx: %+v", err)
			return nil, nil
		}
		return resTx, nil
	}
}

// buildPortalRefundPortingFeeTx builds portal refund porting fee tx
func (curView *ShardBestState) buildPortalRefundPortingFeeTx(
	contentStr string,
	producerPrivateKey *privacy.PrivateKey,
	shardID byte,
) (metadata.Transaction, error) {
	Logger.log.Info("[Portal refund porting fee] Starting...")
	contentBytes := []byte(contentStr)
	var portalPortingRequest metadata.PortalPortingRequestContent
	err := json.Unmarshal(contentBytes, &portalPortingRequest)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshaling portal porting request content: %+v", err)
		return nil, nil
	}
	if portalPortingRequest.ShardID != shardID {
		return nil, nil
	}

	meta := metadata.NewPortalFeeRefundResponse(
		common.PortalPortingRequestRejectedChainStatus,
		portalPortingRequest.TxReqID,
		metadata.PortalPortingResponseMeta,
	)

	keyWallet, err := wallet.Base58CheckDeserialize(portalPortingRequest.IncogAddressStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while deserializing receiver address string: %+v", err)
		return nil, nil
	}
	receiverAddr := keyWallet.KeySet.PaymentAddress

	// the returned currency is PRV
	resTx := new(transaction.Tx)
	err = resTx.InitTxSalary(
		portalPortingRequest.PortingFee,
		&receiverAddr,
		producerPrivateKey,
		curView.GetCopiedTransactionStateDB(),
		meta,
	)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while initializing portal refund porting fee tx: %+v", err)
		return nil, nil
	}
	Logger.log.Info("[Portal refund porting fee] Finished...")
	return resTx, nil
}
