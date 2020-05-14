package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/wallet"
	"strconv"
)

type PortalRedeemRequestResponse struct {
	MetadataBase
	RequestStatus    string
	ReqTxID          common.Hash
	RequesterAddrStr string
	Amount           uint64
	IncTokenID       string
}

func NewPortalRedeemRequestResponse(
	requestStatus string,
	reqTxID common.Hash,
	requesterAddressStr string,
	amount uint64,
	tokenID string,
	metaType int,
) *PortalRedeemRequestResponse {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	return &PortalRedeemRequestResponse{
		RequestStatus:    requestStatus,
		ReqTxID:          reqTxID,
		MetadataBase:     metadataBase,
		RequesterAddrStr: requesterAddressStr,
		Amount:           amount,
		IncTokenID:       tokenID,
	}
}

func (iRes PortalRedeemRequestResponse) CheckTransactionFee(tr Transaction, minFee uint64, beaconHeight int64, db *statedb.StateDB) bool {
	// no need to have fee for this tx
	return true
}

func (iRes PortalRedeemRequestResponse) ValidateTxWithBlockChain(txr Transaction, chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, shardID byte, db *statedb.StateDB) (bool, error) {
	// no need to validate tx with blockchain, just need to validate with requested tx (via RequestedTxID)
	return false, nil
}

func (iRes PortalRedeemRequestResponse) ValidateSanityData(chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, beaconHeight uint64, txr Transaction) (bool, bool, error) {
	return false, true, nil
}

func (iRes PortalRedeemRequestResponse) ValidateMetadataByItself() bool {
	// The validation just need to check at tx level, so returning true here
	return iRes.Type == PortalRedeemRequestResponseMeta
}

func (iRes PortalRedeemRequestResponse) Hash() *common.Hash {
	record := iRes.MetadataBase.Hash().String()
	record += iRes.RequestStatus
	record += iRes.ReqTxID.String()
	record += iRes.RequesterAddrStr
	record += strconv.FormatUint(iRes.Amount, 10)
	record += iRes.IncTokenID
	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (iRes *PortalRedeemRequestResponse) CalculateSize() uint64 {
	return calculateSize(iRes)
}

func (iRes PortalRedeemRequestResponse) VerifyMinerCreatedTxBeforeGettingInBlock(
	txsInBlock []Transaction,
	txsUsed []int,
	insts [][]string,
	instUsed []int,
	shardID byte,
	tx Transaction,
	chainRetriever ChainRetriever,
	ac *AccumulatedValues,
	shardViewRetriever ShardViewRetriever,
	beaconViewRetriever BeaconViewRetriever,
) (bool, error) {
	idx := -1
	for i, inst := range insts {
		if len(inst) < 4 { // this is not PortalRedeemRequest response instruction
			continue
		}
		instMetaType := inst[0]
		if instUsed[i] > 0 ||
			instMetaType != strconv.Itoa(PortalRedeemRequestMeta) {
			continue
		}
		instReqStatus := inst[2]
		if iRes.RequestStatus != "rejected" ||
			(instReqStatus != common.PortalRedeemRequestRejectedChainStatus && instReqStatus != common.PortalRedeemRequestRejectedByLiquidationChainStatus) {
			continue
		}

		var shardIDFromInst byte
		var txReqIDFromInst common.Hash
		var requesterAddrStrFromInst string
		var redeemAmountFromInst uint64
		var tokenIDStrFromInst string

		contentBytes := []byte(inst[3])
		var redeemReqContent PortalRedeemRequestContent
		err := json.Unmarshal(contentBytes, &redeemReqContent)
		if err != nil {
			Logger.log.Error("WARNING - VALIDATION: an error occured while parsing portal redeem request content: ", err)
			continue
		}
		shardIDFromInst = redeemReqContent.ShardID
		txReqIDFromInst = redeemReqContent.TxReqID
		requesterAddrStrFromInst = redeemReqContent.RedeemerIncAddressStr
		redeemAmountFromInst = redeemReqContent.RedeemAmount
		tokenIDStrFromInst = redeemReqContent.TokenID

		if !bytes.Equal(iRes.ReqTxID[:], txReqIDFromInst[:]) ||
			shardID != shardIDFromInst {
			continue
		}
		if requesterAddrStrFromInst != iRes.RequesterAddrStr {
			Logger.log.Errorf("Error - VALIDATION: Requester address %v is not matching to Requester address in instruction %v", iRes.RequesterAddrStr, requesterAddrStrFromInst)
			continue
		}

		if redeemAmountFromInst != iRes.Amount {
			Logger.log.Errorf("Error - VALIDATION: Redeem amount %v is not matching to redeem amount in instruction %v", iRes.Amount, redeemAmountFromInst)
			continue
		}

		key, err := wallet.Base58CheckDeserialize(requesterAddrStrFromInst)
		if err != nil {
			Logger.log.Info("WARNING - VALIDATION: an error occured while deserializing requester address string: ", err)
			continue
		}

		_, pk, paidAmount, assetID := tx.GetTransferData()
		if !bytes.Equal(key.KeySet.PaymentAddress.Pk[:], pk[:]) ||
			redeemAmountFromInst != paidAmount ||
			tokenIDStrFromInst != assetID.String() {
			continue
		}
		idx = i
		break
	}
	if idx == -1 { // not found the issuance request tx for this response
		return false, fmt.Errorf(fmt.Sprintf("no PortalRedeemRequest instruction found for PortalRedeemRequestResponse tx %s", tx.Hash().String()))
	}
	instUsed[idx] = 1
	return true, nil
}
