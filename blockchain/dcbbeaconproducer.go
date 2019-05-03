package blockchain

import (
	"fmt"
	"strconv"

	"github.com/constant-money/constant-chain/blockchain/component"
	"github.com/constant-money/constant-chain/common"
	"github.com/constant-money/constant-chain/metadata"
	"github.com/constant-money/constant-chain/privacy"
	"github.com/constant-money/constant-chain/wallet"
)

// buildPassThroughInstruction converts shard instruction to beacon instruction in order to update BeaconBestState later on in beaconprocess
func buildPassThroughInstruction(receivedType int, contentStr string) ([][]string, error) {
	metaType := strconv.Itoa(receivedType)
	shardID := strconv.Itoa(component.BeaconOnly)
	return [][]string{[]string{metaType, shardID, contentStr}}, nil
}

func buildInstructionsForCrowdsaleRequest(
	shardID byte,
	contentStr string,
	beaconBestState *BestStateBeacon,
	accumulativeValues *accumulativeValues,
) ([][]string, error) {
	saleID, priceLimit, limitSell, paymentAddress, sentAmount, err := metadata.ParseCrowdsaleRequestActionValue(contentStr)
	if err != nil {
		// fmt.Printf("[db] error parsing action: %+v\n", err)
		return nil, err
	}

	// Get data of current crowdsale
	key := getSaleDataKeyBeacon(saleID)
	var saleData *component.SaleData
	ok := false
	if saleData, ok = accumulativeValues.saleDataMap[key]; !ok {
		if value, ok := beaconBestState.Params[key]; ok {
			saleData, err = parseSaleDataValueBeacon(value)
			if err != nil {
				return nil, fmt.Errorf("fail parsing SaleData: %v", err)
			}
		} else {
			// fmt.Printf("[db] saleid not exist: %x\n", saleID)
			return nil, fmt.Errorf("saleID not exist: %x", saleID)
		}
	}
	accumulativeValues.saleDataMap[key] = saleData

	inst, err := buildPaymentInstructionForCrowdsale(
		priceLimit,
		limitSell,
		paymentAddress,
		sentAmount,
		beaconBestState,
		saleData,
	)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func buildPaymentInstructionForCrowdsale(
	priceLimit uint64,
	limitSell bool,
	paymentAddress privacy.PaymentAddress,
	sentAmount uint64,
	beaconBestState *BestStateBeacon,
	saleData *component.SaleData,
) ([][]string, error) {
	// Get price for asset
	buyingAsset := saleData.BuyingAsset
	sellingAsset := saleData.SellingAsset
	buyPrice := beaconBestState.GetAssetPrice(buyingAsset)
	sellPrice := beaconBestState.GetAssetPrice(sellingAsset)
	if buyPrice == 0 {
		buyPrice = saleData.DefaultBuyPrice
	}
	if sellPrice == 0 {
		sellPrice = saleData.DefaultSellPrice
	}
	if buyPrice == 0 || sellPrice == 0 {
		// fmt.Printf("[db] asset price is 0: %d %d\n", buyPrice, sellPrice)
		return generateCrowdsalePaymentInstruction(paymentAddress, sentAmount, buyingAsset, saleData.SaleID, 0, false) // refund
	}
	// fmt.Printf("[db] buy and sell price: %d %d\n", buyPrice, sellPrice)

	// Check if price limit is not violated
	if limitSell && sellPrice > priceLimit {
		// fmt.Printf("[db] Price limit violated: %d %d\n", sellPrice, priceLimit)
		return generateCrowdsalePaymentInstruction(paymentAddress, sentAmount, buyingAsset, saleData.SaleID, 0, false) // refund
	} else if !limitSell && buyPrice < priceLimit {
		// fmt.Printf("[db] Price limit violated: %d %d\n", buyPrice, priceLimit)
		return generateCrowdsalePaymentInstruction(paymentAddress, sentAmount, buyingAsset, saleData.SaleID, 0, false) // refund
	}

	// Check if sale is on-going
	if bestStateBeacon.BeaconHeight >= saleData.EndBlock {
		return generateCrowdsalePaymentInstruction(paymentAddress, sentAmount, buyingAsset, saleData.SaleID, 0, false) // refund
	}

	// Calculate value of asset sent in request tx
	sentAssetValue := sentAmount * buyPrice // in cent
	if common.IsConstantAsset(&saleData.BuyingAsset) {
		sentAssetValue /= 100 // Nano to CST
	}

	// Number of asset must pay to user
	paymentAmount := sentAssetValue / sellPrice
	if common.IsConstantAsset(&saleData.SellingAsset) {
		paymentAmount *= 100 // CST to Nano
	}

	// Check if there's still enough asset to trade
	if sentAmount > saleData.BuyingAmount || paymentAmount > saleData.SellingAmount {
		// fmt.Printf("[db] Crowdsale reached limit\n")
		return generateCrowdsalePaymentInstruction(paymentAddress, sentAmount, buyingAsset, saleData.SaleID, 0, false) // refund
	}

	// Update amount of buying/selling asset of the crowdsale
	saleData.BuyingAmount -= sentAmount
	saleData.SellingAmount -= paymentAmount

	// fmt.Printf("[db] sentValue, payAmount, buyLeft, sellLeft: %d %d %d %d\n", sentAssetValue, paymentAmount, saleData.BuyingAmount, saleData.SellingAmount)

	// Build instructions
	return generateCrowdsalePaymentInstruction(paymentAddress, paymentAmount, sellingAsset, saleData.SaleID, sentAmount, true)
}

func buildInstructionsForTradeActivation(
	shardID byte,
	contentStr string,
) ([][]string, error) {
	keyWalletDCBAccount, _ := wallet.Base58CheckDeserialize(common.DCBAddress)
	dcbPk := keyWalletDCBAccount.KeySet.PaymentAddress.Pk
	dcbShardID := common.GetShardIDFromLastByte(dcbPk[len(dcbPk)-1])
	inst := []string{
		strconv.Itoa(metadata.TradeActivationMeta),
		strconv.Itoa(int(dcbShardID)),
		contentStr,
	}
	fmt.Printf("[db] beacon built inst: %v\n", inst)
	return [][]string{inst}, nil
}
