package blockchain

import (
	"encoding/base64"
	"encoding/json"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"math/big"
	"sort"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
)

// build instructions at beacon chain before syncing to shards
func (blockchain *BlockChain) collectStatefulActions(
	shardBlockInstructions [][]string,
) [][]string {
	// stateful instructions are dependently processed with results of instructioins before them in shards2beacon blocks
	statefulInsts := [][]string{}
	for _, inst := range shardBlockInstructions {
		if len(inst) < 2 {
			continue
		}
		if inst[0] == SetAction || inst[0] == StakeAction || inst[0] == SwapAction || inst[0] == RandomAction || inst[0] == AssignAction {
			continue
		}

		metaType, err := strconv.Atoi(inst[0])
		if err != nil {
			Logger.log.Error(err)
			continue
		}
		switch metaType {
		case metadata.IssuingRequestMeta,
			metadata.IssuingETHRequestMeta,
			metadata.PDEContributionMeta,
			metadata.PDETradeRequestMeta,
			metadata.PDEWithdrawalRequestMeta,
			metadata.PortalCustodianDepositMeta,
			metadata.PortalUserRegisterMeta,
			metadata.PortalUserRequestPTokenMeta,
			metadata.PortalExchangeRatesMeta,
			metadata.RelayingBNBHeaderMeta,
			metadata.RelayingBTCHeaderMeta,
			metadata.PortalCustodianWithdrawRequestMeta,
			metadata.PortalRedeemRequestMeta,
			metadata.PortalRequestUnlockCollateralMeta,
			metadata.PortalLiquidateCustodianMeta,
			metadata.PortalRequestWithdrawRewardMeta,
			metadata.PortalRedeemLiquidateExchangeRatesMeta,
			metadata.PortalLiquidationCustodianDepositMeta,
			metadata.PortalLiquidationCustodianDepositResponseMeta:
			statefulInsts = append(statefulInsts, inst)

		default:
			continue
		}
	}
	return statefulInsts
}

func groupPDEActionsByShardID(
	pdeActionsByShardID map[byte][][]string,
	action []string,
	shardID byte,
) map[byte][][]string {
	_, found := pdeActionsByShardID[shardID]
	if !found {
		pdeActionsByShardID[shardID] = [][]string{action}
	} else {
		pdeActionsByShardID[shardID] = append(pdeActionsByShardID[shardID], action)
	}
	return pdeActionsByShardID
}

func (blockchain *BlockChain) buildStatefulInstructions(
	stateDB *statedb.StateDB,
	statefulActionsByShardID map[byte][][]string,
	beaconHeight uint64,
	rewardForCustodianByEpoch map[common.Hash]uint64,
	portalParams PortalParams) [][]string {
	currentPDEState, err := InitCurrentPDEStateFromDB(stateDB, beaconHeight-1)
	if err != nil {
		Logger.log.Error(err)
	}

	currentPortalState, err := InitCurrentPortalStateFromDB(stateDB)
	if err != nil {
		Logger.log.Error(err)
	}

	pm := NewPortalManager()
	relayingHeaderState, err := blockchain.InitRelayingHeaderChainStateFromDB()
	if err != nil {
		Logger.log.Error(err)
	}

	accumulatedValues := &metadata.AccumulatedValues{
		UniqETHTxsUsed:   [][]byte{},
		DBridgeTokenPair: map[string][]byte{},
		CBridgeTokens:    []*common.Hash{},
	}
	instructions := [][]string{}

	// pde instructions
	pdeContributionActionsByShardID := map[byte][][]string{}
	pdeTradeActionsByShardID := map[byte][][]string{}
	pdeWithdrawalActionsByShardID := map[byte][][]string{}

	// portal instructions
	portalCustodianDepositActionsByShardID := map[byte][][]string{}
	portalUserReqPortingActionsByShardID := map[byte][][]string{}
	portalUserReqPTokenActionsByShardID := map[byte][][]string{}
	portalExchangeRatesActionsByShardID := map[byte][][]string{}
	portalRedeemReqActionsByShardID := map[byte][][]string{}
	portalCustodianWithdrawActionsByShardID := map[byte][][]string{}
	portalReqUnlockCollateralActionsByShardID := map[byte][][]string{}
	portalReqWithdrawRewardActionsByShardID := map[byte][][]string{}
	portalRedeemLiquidateExchangeRatesActionByShardID := map[byte][][]string{}
	portalLiquidationCustodianDepositActionByShardID := map[byte][][]string{}

	var keys []int
	for k := range statefulActionsByShardID {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, value := range keys {
		shardID := byte(value)
		actions := statefulActionsByShardID[shardID]
		for _, action := range actions {
			metaType, err := strconv.Atoi(action[0])
			if err != nil {
				continue
			}
			contentStr := action[1]
			newInst := [][]string{}
			switch metaType {
			case metadata.IssuingRequestMeta:
				newInst, err = blockchain.buildInstructionsForIssuingReq(stateDB, contentStr, shardID, metaType, accumulatedValues)

			case metadata.IssuingETHRequestMeta:
				newInst, err = blockchain.buildInstructionsForIssuingETHReq(stateDB, contentStr, shardID, metaType, accumulatedValues)

			case metadata.PDEContributionMeta:
				pdeContributionActionsByShardID = groupPDEActionsByShardID(
					pdeContributionActionsByShardID,
					action,
					shardID,
				)
			case metadata.PDETradeRequestMeta:
				pdeTradeActionsByShardID = groupPDEActionsByShardID(
					pdeTradeActionsByShardID,
					action,
					shardID,
				)
			case metadata.PDEWithdrawalRequestMeta:
				pdeWithdrawalActionsByShardID = groupPDEActionsByShardID(
					pdeWithdrawalActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalCustodianDepositMeta:
				{
					portalCustodianDepositActionsByShardID = groupPortalActionsByShardID(
						portalCustodianDepositActionsByShardID,
						action,
						shardID,
					)
				}

			case metadata.PortalUserRegisterMeta:
				portalUserReqPortingActionsByShardID = groupPortalActionsByShardID(
					portalUserReqPortingActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalUserRequestPTokenMeta:
				portalUserReqPTokenActionsByShardID = groupPortalActionsByShardID(
					portalUserReqPTokenActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalExchangeRatesMeta:
				portalExchangeRatesActionsByShardID = groupPortalActionsByShardID(
					portalExchangeRatesActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalCustodianWithdrawRequestMeta:
				portalCustodianWithdrawActionsByShardID = groupPortalActionsByShardID(
					portalCustodianWithdrawActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalRedeemRequestMeta:
				portalRedeemReqActionsByShardID = groupPortalActionsByShardID(
					portalRedeemReqActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalRequestUnlockCollateralMeta:
				portalReqUnlockCollateralActionsByShardID = groupPortalActionsByShardID(
					portalReqUnlockCollateralActionsByShardID,
					action,
					shardID,
				)
			case metadata.PortalRequestWithdrawRewardMeta:
				portalReqWithdrawRewardActionsByShardID = groupPortalActionsByShardID(
					portalReqWithdrawRewardActionsByShardID,
					action,
					shardID,
				)

			case metadata.PortalRedeemLiquidateExchangeRatesMeta:
				portalRedeemLiquidateExchangeRatesActionByShardID = groupPortalActionsByShardID(
					portalRedeemLiquidateExchangeRatesActionByShardID,
					action,
					shardID,
				)
			case metadata.PortalLiquidationCustodianDepositMeta:
				portalLiquidationCustodianDepositActionByShardID = groupPortalActionsByShardID(
					portalLiquidationCustodianDepositActionByShardID,
					action,
					shardID,
				)
			case metadata.RelayingBNBHeaderMeta:
				pm.relayingChains[metadata.RelayingBNBHeaderMeta].putAction(action)
			case metadata.RelayingBTCHeaderMeta:
				pm.relayingChains[metadata.RelayingBTCHeaderMeta].putAction(action)
			default:
				continue
			}
			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	pdeInsts, err := blockchain.handlePDEInsts(
		beaconHeight-1, currentPDEState,
		pdeContributionActionsByShardID,
		pdeTradeActionsByShardID,
		pdeWithdrawalActionsByShardID,
	)

	if err != nil {
		Logger.log.Error(err)
		return instructions
	}
	if len(pdeInsts) > 0 {
		instructions = append(instructions, pdeInsts...)
	}

	// auto-liquidation portal instructions
	portalLiquidationInsts, err := blockchain.autoCheckAndCreatePortalLiquidationInsts(
		beaconHeight-1,
		currentPortalState,
		portalParams,
	)

	if err != nil {
		Logger.log.Error(err)
		return instructions
	}
	if len(portalLiquidationInsts) > 0 {
		instructions = append(instructions, portalLiquidationInsts...)
	}

	// handle portal instructions
	portalInsts, err := blockchain.handlePortalInsts(
		stateDB,
		beaconHeight-1,
		currentPortalState,
		portalCustodianDepositActionsByShardID,
		portalUserReqPortingActionsByShardID,
		portalUserReqPTokenActionsByShardID,
		portalExchangeRatesActionsByShardID,
		portalRedeemReqActionsByShardID,
		portalCustodianWithdrawActionsByShardID,
		portalReqUnlockCollateralActionsByShardID,
		portalRedeemLiquidateExchangeRatesActionByShardID,
		portalLiquidationCustodianDepositActionByShardID,
		portalParams,
	)

	if err != nil {
		Logger.log.Error(err)
		return instructions
	}
	if len(portalInsts) > 0 {
		instructions = append(instructions, portalInsts...)
	}

	// handle relaying instructions
	relayingInsts := blockchain.handleRelayingInsts(relayingHeaderState, pm)
	if len(relayingInsts) > 0 {
		instructions = append(instructions, relayingInsts...)
	}

	// calculate rewards (include porting fee and redeem fee) for custodians and build instructions at beaconHeight
	portalRewardsInsts, err := blockchain.handlePortalRewardInsts(
		beaconHeight-1,
		currentPortalState,
		portalReqWithdrawRewardActionsByShardID,
		rewardForCustodianByEpoch,
	)

	if err != nil {
		Logger.log.Error(err)
		return instructions
	}
	if len(portalRewardsInsts) > 0 {
		instructions = append(instructions, portalRewardsInsts...)
	}

	return instructions
}

func sortPDETradeInstsByFee(
	beaconHeight uint64,
	currentPDEState *CurrentPDEState,
	pdeTradeActionsByShardID map[byte][][]string,
) []metadata.PDETradeRequestAction {
	tradesByPairs := make(map[string][]metadata.PDETradeRequestAction)

	var keys []int
	for k := range pdeTradeActionsByShardID {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, value := range keys {
		shardID := byte(value)
		actions := pdeTradeActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			contentBytes, err := base64.StdEncoding.DecodeString(contentStr)
			if err != nil {
				Logger.log.Errorf("ERROR: an error occured while decoding content string of pde trade action: %+v", err)
				continue
			}
			var pdeTradeReqAction metadata.PDETradeRequestAction
			err = json.Unmarshal(contentBytes, &pdeTradeReqAction)
			if err != nil {
				Logger.log.Errorf("ERROR: an error occured while unmarshaling pde trade action: %+v", err)
				continue
			}
			tradeMeta := pdeTradeReqAction.Meta
			poolPairKey := string(rawdbv2.BuildPDEPoolForPairKey(beaconHeight, tradeMeta.TokenIDToBuyStr, tradeMeta.TokenIDToSellStr))
			tradesByPair, found := tradesByPairs[poolPairKey]
			if !found {
				tradesByPairs[poolPairKey] = []metadata.PDETradeRequestAction{pdeTradeReqAction}
			} else {
				tradesByPairs[poolPairKey] = append(tradesByPair, pdeTradeReqAction)
			}
		}
	}

	notExistingPairTradeActions := []metadata.PDETradeRequestAction{}
	sortedExistingPairTradeActions := []metadata.PDETradeRequestAction{}

	var ppKeys []string
	for k := range tradesByPairs {
		ppKeys = append(ppKeys, k)
	}
	sort.Strings(ppKeys)
	for _, poolPairKey := range ppKeys {
		tradeActions := tradesByPairs[poolPairKey]
		poolPair, found := currentPDEState.PDEPoolPairs[poolPairKey]
		if !found || poolPair == nil {
			notExistingPairTradeActions = append(notExistingPairTradeActions, tradeActions...)
			continue
		}
		if poolPair.Token1PoolValue == 0 || poolPair.Token2PoolValue == 0 {
			notExistingPairTradeActions = append(notExistingPairTradeActions, tradeActions...)
			continue
		}

		// sort trade actions by trading fee
		sort.Slice(tradeActions, func(i, j int) bool {
			// comparing a/b to c/d is equivalent with comparing a*d to c*b
			firstItemProportion := big.NewInt(0)
			firstItemProportion.Mul(
				big.NewInt(int64(tradeActions[i].Meta.TradingFee)),
				big.NewInt(int64(tradeActions[j].Meta.SellAmount)),
			)
			secondItemProportion := big.NewInt(0)
			secondItemProportion.Mul(
				big.NewInt(int64(tradeActions[j].Meta.TradingFee)),
				big.NewInt(int64(tradeActions[i].Meta.SellAmount)),
			)
			return firstItemProportion.Cmp(secondItemProportion) == 1
		})
		sortedExistingPairTradeActions = append(sortedExistingPairTradeActions, tradeActions...)
	}
	return append(sortedExistingPairTradeActions, notExistingPairTradeActions...)
}

func (blockchain *BlockChain) handlePDEInsts(
	beaconHeight uint64,
	currentPDEState *CurrentPDEState,
	pdeContributionActionsByShardID map[byte][][]string,
	pdeTradeActionsByShardID map[byte][][]string,
	pdeWithdrawalActionsByShardID map[byte][][]string,
) ([][]string, error) {
	instructions := [][]string{}
	sortedTradesActions := sortPDETradeInstsByFee(
		beaconHeight,
		currentPDEState,
		pdeTradeActionsByShardID,
	)
	for _, tradeAction := range sortedTradesActions {
		actionContentBytes, _ := json.Marshal(tradeAction)
		actionContentBase64Str := base64.StdEncoding.EncodeToString(actionContentBytes)
		newInst, err := blockchain.buildInstructionsForPDETrade(actionContentBase64Str, tradeAction.ShardID, metadata.PDETradeRequestMeta, currentPDEState, beaconHeight)
		if err != nil {
			Logger.log.Error(err)
			continue
		}
		if len(newInst) > 0 {
			instructions = append(instructions, newInst...)
		}
	}

	// handle withdrawal
	var wrKeys []int
	for k := range pdeWithdrawalActionsByShardID {
		wrKeys = append(wrKeys, int(k))
	}
	sort.Ints(wrKeys)
	for _, value := range wrKeys {
		shardID := byte(value)
		actions := pdeWithdrawalActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForPDEWithdrawal(contentStr, shardID, metadata.PDEWithdrawalRequestMeta, currentPDEState, beaconHeight)
			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle contribution
	var ctKeys []int
	for k := range pdeContributionActionsByShardID {
		ctKeys = append(ctKeys, int(k))
	}
	sort.Ints(ctKeys)
	for _, value := range ctKeys {
		shardID := byte(value)
		actions := pdeContributionActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForPDEContribution(contentStr, shardID, metadata.PDEContributionMeta, currentPDEState, beaconHeight)
			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}
	return instructions, nil
}

// Portal
func groupPortalActionsByShardID(
	portalActionsByShardID map[byte][][]string,
	action []string,
	shardID byte,
) map[byte][][]string {
	_, found := portalActionsByShardID[shardID]
	if !found {
		portalActionsByShardID[shardID] = [][]string{action}
	} else {
		portalActionsByShardID[shardID] = append(portalActionsByShardID[shardID], action)
	}
	return portalActionsByShardID
}

func (blockchain *BlockChain) handlePortalInsts(
	stateDB *statedb.StateDB,
	beaconHeight uint64,
	currentPortalState *CurrentPortalState,
	portalCustodianDepositActionsByShardID map[byte][][]string,
	portalUserRequestPortingActionsByShardID map[byte][][]string,
	portalUserRequestPTokenActionsByShardID map[byte][][]string,
	portalExchangeRatesActionsByShardID map[byte][][]string,
	portalRedeemReqActionsByShardID map[byte][][]string,
	portalCustodianWithdrawActionByShardID map[byte][][]string,
	portalReqUnlockCollateralActionsByShardID map[byte][][]string,
	portalRedeemLiquidateExchangeRatesActionByShardID map[byte][][]string,
	portalLiquidationCustodianDepositActionByShardID map[byte][][]string,
	portalParams PortalParams,
) ([][]string, error) {
	instructions := [][]string{}

	// handle portal custodian deposit inst
	var custodianShardIDKeys []int
	for k := range portalCustodianDepositActionsByShardID {
		custodianShardIDKeys = append(custodianShardIDKeys, int(k))
	}

	sort.Ints(custodianShardIDKeys)
	for _, value := range custodianShardIDKeys {
		shardID := byte(value)
		actions := portalCustodianDepositActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForCustodianDeposit(
				contentStr,
				shardID,
				metadata.PortalCustodianDepositMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle portal user request porting inst
	var requestPortingShardIDKeys []int
	for k := range portalUserRequestPortingActionsByShardID {
		requestPortingShardIDKeys = append(requestPortingShardIDKeys, int(k))
	}

	sort.Ints(requestPortingShardIDKeys)
	for _, value := range requestPortingShardIDKeys {
		shardID := byte(value)
		actions := portalUserRequestPortingActionsByShardID[shardID]

		//check identity of porting request id
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForPortingRequest(
				stateDB,
				contentStr,
				shardID,
				metadata.PortalUserRegisterMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}
	// handle portal user request ptoken inst
	var reqPTokenShardIDKeys []int
	for k := range portalUserRequestPTokenActionsByShardID {
		reqPTokenShardIDKeys = append(reqPTokenShardIDKeys, int(k))
	}

	sort.Ints(reqPTokenShardIDKeys)
	for _, value := range reqPTokenShardIDKeys {
		shardID := byte(value)
		actions := portalUserRequestPTokenActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForReqPTokens(
				stateDB,
				contentStr,
				shardID,
				metadata.PortalUserRequestPTokenMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle portal redeem req inst
	var redeemReqShardIDKeys []int
	for k := range portalRedeemReqActionsByShardID {
		redeemReqShardIDKeys = append(redeemReqShardIDKeys, int(k))
	}

	sort.Ints(redeemReqShardIDKeys)
	for _, value := range redeemReqShardIDKeys {
		shardID := byte(value)
		actions := portalRedeemReqActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForRedeemRequest(
				stateDB,
				contentStr,
				shardID,
				metadata.PortalRedeemRequestMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	//handle portal exchange rates
	var exchangeRatesShardIDKeys []int
	for k := range portalExchangeRatesActionsByShardID {
		exchangeRatesShardIDKeys = append(exchangeRatesShardIDKeys, int(k))
	}

	sort.Ints(exchangeRatesShardIDKeys)
	for _, value := range exchangeRatesShardIDKeys {
		shardID := byte(value)
		actions := portalExchangeRatesActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForExchangeRates(
				contentStr,
				shardID,
				metadata.PortalExchangeRatesMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	//handle portal custodian withdraw
	var portalCustodianWithdrawShardIDKeys []int
	for k := range portalCustodianWithdrawActionByShardID {
		portalCustodianWithdrawShardIDKeys = append(portalCustodianWithdrawShardIDKeys, int(k))
	}

	sort.Ints(portalCustodianWithdrawShardIDKeys)
	for _, value := range portalCustodianWithdrawShardIDKeys {
		shardID := byte(value)
		actions := portalCustodianWithdrawActionByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForCustodianWithdraw(
				contentStr,
				shardID,
				metadata.PortalCustodianWithdrawRequestMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle portal req unlock collateral inst
	var reqUnlockCollateralShardIDKeys []int
	for k := range portalReqUnlockCollateralActionsByShardID {
		reqUnlockCollateralShardIDKeys = append(reqUnlockCollateralShardIDKeys, int(k))
	}

	sort.Ints(reqUnlockCollateralShardIDKeys)
	for _, value := range reqUnlockCollateralShardIDKeys {
		shardID := byte(value)
		actions := portalReqUnlockCollateralActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForReqUnlockCollateral(
				stateDB,
				contentStr,
				shardID,
				metadata.PortalRequestUnlockCollateralMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle liquidation user redeem ptoken  exchange rates
	var redeemLiquidateExchangeRatesActionByShardIDKeys []int
	for k := range portalRedeemLiquidateExchangeRatesActionByShardID {
		redeemLiquidateExchangeRatesActionByShardIDKeys = append(redeemLiquidateExchangeRatesActionByShardIDKeys, int(k))
	}

	sort.Ints(redeemLiquidateExchangeRatesActionByShardIDKeys)
	for _, value := range redeemLiquidateExchangeRatesActionByShardIDKeys {
		shardID := byte(value)
		actions := portalRedeemLiquidateExchangeRatesActionByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForLiquidationRedeemPTokenExchangeRates(
				contentStr,
				shardID,
				metadata.PortalRedeemLiquidateExchangeRatesMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	// handle portal  liquidation custodian deposit inst
	var portalLiquidationCustodianDepositActionByShardIDKeys []int
	for k := range portalLiquidationCustodianDepositActionByShardID {
		portalLiquidationCustodianDepositActionByShardIDKeys = append(portalLiquidationCustodianDepositActionByShardIDKeys, int(k))
	}

	sort.Ints(portalLiquidationCustodianDepositActionByShardIDKeys)
	for _, value := range portalLiquidationCustodianDepositActionByShardIDKeys {
		shardID := byte(value)
		actions := portalLiquidationCustodianDepositActionByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForLiquidationCustodianDeposit(
				contentStr,
				shardID,
				metadata.PortalLiquidationCustodianDepositMeta,
				currentPortalState,
				beaconHeight,
				portalParams,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	return instructions, nil
}

// Header relaying
func groupRelayingActionsByShardID(
	relayingActionsByShardID map[byte][][]string,
	action []string,
	shardID byte,
) map[byte][][]string {
	_, found := relayingActionsByShardID[shardID]
	if !found {
		relayingActionsByShardID[shardID] = [][]string{action}
	} else {
		relayingActionsByShardID[shardID] = append(relayingActionsByShardID[shardID], action)
	}
	return relayingActionsByShardID
}

func (blockchain *BlockChain) autoCheckAndCreatePortalLiquidationInsts(
	beaconHeight uint64,
	currentPortalState *CurrentPortalState,
	portalParams PortalParams) ([][]string, error) {
	//Logger.log.Errorf("autoCheckAndCreatePortalLiquidationInsts starting.......")

	insts := [][]string{}

	// check there is any waiting porting request timeout
	expiredWaitingPortingInsts, err := blockchain.checkAndBuildInstForExpiredWaitingPortingRequest(beaconHeight, currentPortalState, portalParams)
	if err != nil {
		Logger.log.Errorf("Error when check and build custodian liquidation %v\n", err)
	}
	if len(expiredWaitingPortingInsts) > 0 {
		insts = append(insts, expiredWaitingPortingInsts...)
	}
	Logger.log.Infof("There are %v instruction for expired waiting porting in portal\n", len(expiredWaitingPortingInsts))

	// case 1: check there is any custodian doesn't send public tokens back to user after TimeOutCustodianReturnPubToken
	// get custodian's collateral to return user
	custodianLiqInsts, err := blockchain.checkAndBuildInstForCustodianLiquidation(beaconHeight, currentPortalState, portalParams)
	if err != nil {
		Logger.log.Errorf("Error when check and build custodian liquidation %v\n", err)
	}
	if len(custodianLiqInsts) > 0 {
		insts = append(insts, custodianLiqInsts...)
	}
	Logger.log.Infof("There are %v instruction for custodian liquidation in portal\n", len(custodianLiqInsts))

	// case 2: check collateral's value (locked collateral amount) drops below MinRatio

	exchangeRatesLiqInsts, err := buildInstForLiquidationTopPercentileExchangeRates(beaconHeight, currentPortalState, portalParams)
	if err != nil {
		Logger.log.Errorf("Error when check and build exchange rates liquidation %v\n", err)
	}
	if len(exchangeRatesLiqInsts) > 0 {
		insts = append(insts, exchangeRatesLiqInsts...)
	}

	Logger.log.Infof("There are %v instruction for exchange rates liquidation in portal\n", len(exchangeRatesLiqInsts))

	return insts, nil
}

// handlePortalRewardInsts
// 1. Build instructions for request withdraw portal reward
// 2. Build instructions portal reward for each beacon block
func (blockchain *BlockChain) handlePortalRewardInsts(
	beaconHeight uint64,
	currentPortalState *CurrentPortalState,
	portalReqWithdrawRewardActionsByShardID map[byte][][]string,
	rewardForCustodianByEpoch map[common.Hash]uint64,
) ([][]string, error) {
	instructions := [][]string{}

	// Build instructions portal reward for each beacon block
	portalRewardInsts, err := blockchain.buildPortalRewardsInsts(beaconHeight, currentPortalState, rewardForCustodianByEpoch)
	if err != nil {
		Logger.log.Error(err)
	}
	if len(portalRewardInsts) > 0 {
		instructions = append(instructions, portalRewardInsts...)
	}

	// handle portal request withdraw reward inst
	var shardIDKeys []int
	for k := range portalReqWithdrawRewardActionsByShardID {
		shardIDKeys = append(shardIDKeys, int(k))
	}

	sort.Ints(shardIDKeys)
	for _, value := range shardIDKeys {
		shardID := byte(value)
		actions := portalReqWithdrawRewardActionsByShardID[shardID]
		for _, action := range actions {
			contentStr := action[1]
			newInst, err := blockchain.buildInstructionsForReqWithdrawPortalReward(
				contentStr,
				shardID,
				metadata.PortalRequestWithdrawRewardMeta,
				currentPortalState,
				beaconHeight,
			)

			if err != nil {
				Logger.log.Error(err)
				continue
			}
			if len(newInst) > 0 {
				instructions = append(instructions, newInst...)
			}
		}
	}

	return instructions, nil
}
