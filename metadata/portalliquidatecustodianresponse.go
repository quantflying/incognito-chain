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

type PortalLiquidateCustodianResponse struct {
	MetadataBase
	UniqueRedeemID         string
	MintedCollateralAmount uint64 // minted PRV amount for sending back to users
	RedeemerIncAddressStr  string
	CustodianIncAddressStr string
}

func NewPortalLiquidateCustodianResponse(
	uniqueRedeemID string,
	mintedAmount uint64,
	redeemerIncAddressStr string,
	custodianIncAddressStr string,
	metaType int,
) *PortalLiquidateCustodianResponse {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	return &PortalLiquidateCustodianResponse{
		MetadataBase:           metadataBase,
		UniqueRedeemID:         uniqueRedeemID,
		MintedCollateralAmount: mintedAmount,
		RedeemerIncAddressStr:  redeemerIncAddressStr,
		CustodianIncAddressStr: custodianIncAddressStr,
	}
}

func (iRes PortalLiquidateCustodianResponse) CheckTransactionFee(tr Transaction, minFee uint64, beaconHeight int64, db *statedb.StateDB) bool {
	// no need to have fee for this tx
	return true
}

func (iRes PortalLiquidateCustodianResponse) ValidateTxWithBlockChain(txr Transaction, chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, shardID byte, db *statedb.StateDB) (bool, error) {
	// no need to validate tx with blockchain, just need to validate with requested tx (via RequestedTxID)
	return false, nil
}

func (iRes PortalLiquidateCustodianResponse) ValidateSanityData(chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, beaconHeight uint64, txr Transaction) (bool, bool, error) {
	return false, true, nil
}

func (iRes PortalLiquidateCustodianResponse) ValidateMetadataByItself() bool {
	// The validation just need to check at tx level, so returning true here
	return iRes.Type == PortalLiquidateCustodianResponseMeta
}

func (iRes PortalLiquidateCustodianResponse) Hash() *common.Hash {
	record := iRes.UniqueRedeemID
	record += strconv.FormatUint(iRes.MintedCollateralAmount, 10)
	record += iRes.RedeemerIncAddressStr
	record += iRes.CustodianIncAddressStr
	record += iRes.MetadataBase.Hash().String()

	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (iRes *PortalLiquidateCustodianResponse) CalculateSize() uint64 {
	return calculateSize(iRes)
}

func (iRes PortalLiquidateCustodianResponse) VerifyMinerCreatedTxBeforeGettingInBlock(
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
		if len(inst) < 4 { // this is not PortalLiquidateCustodian response instruction
			continue
		}
		instMetaType := inst[0]
		if instUsed[i] > 0 ||
			instMetaType != strconv.Itoa(PortalLiquidateCustodianMeta) {
			continue
		}

		status := inst[2]
		if status != common.PortalLiquidateCustodianSuccessChainStatus {
			continue
		}

		var shardIDFromInst byte
		var custodianAddrStrFromInst string
		var redeemerIncAddressStrFromInst string
		var mintedCollateralAmountFromInst uint64

		contentBytes := []byte(inst[3])
		var liqCustodianContent PortalLiquidateCustodianContent
		err := json.Unmarshal(contentBytes, &liqCustodianContent)
		if err != nil {
			Logger.log.Error("WARNING - VALIDATION: an error occured while parsing portal liquidation custodian content: %v", err)
			continue
		}

		custodianAddrStrFromInst = liqCustodianContent.CustodianIncAddressStr
		redeemerIncAddressStrFromInst = liqCustodianContent.RedeemerIncAddressStr
		mintedCollateralAmountFromInst = liqCustodianContent.LiquidatedCollateralAmount
		shardIDFromInst = liqCustodianContent.ShardID

		if shardIDFromInst != shardID {
			Logger.log.Error("WARNING - VALIDATION: shardID is incorrect: shardIDFromInst %v - shardID %v ", shardIDFromInst, shardID)
			continue
		}

		_, err = wallet.Base58CheckDeserialize(custodianAddrStrFromInst)
		if err != nil {
			Logger.log.Info("WARNING - VALIDATION: an error occured while deserializing custodian address string: ", err)
			continue
		}

		redeemerKey, err := wallet.Base58CheckDeserialize(redeemerIncAddressStrFromInst)
		if err != nil {
			Logger.log.Info("WARNING - VALIDATION: an error occured while deserializing redeemer address string: ", err)
			continue
		}

		// collateral must be PRV
		PRVIDStr := common.PRVCoinID.String()
		_, pk, paidAmount, assetID := tx.GetTransferData()
		if !bytes.Equal(redeemerKey.KeySet.PaymentAddress.Pk[:], pk[:]) ||
			mintedCollateralAmountFromInst != paidAmount ||
			PRVIDStr != assetID.String() {
			continue
		}
		idx = i
		break
	}
	if idx == -1 { // not found the issuance request tx for this response
		return false, fmt.Errorf(fmt.Sprintf("no PortalLiquidateCustodian instruction found for PortalLiquidateCustodianResponse tx %s", tx.Hash().String()))
	}
	instUsed[idx] = 1
	return true, nil
}
