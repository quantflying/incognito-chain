package app

import (
	"strconv"

	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/pkg/errors"
)

func storeBurningConfirm(bridgeStateDB *statedb.StateDB, block blockinterface.ShardBlockInterface) error {
	for _, inst := range block.GetShardBody().GetInstructions() {
		if inst[0] != strconv.Itoa(metadata.BurningConfirmMeta) {
			continue
		}
		Logger.log.Infof("storeBurningConfirm for block %d, inst %v", block.GetShardHeader().GetHeight(), inst)

		txID, err := common.Hash{}.NewHashFromStr(inst[5])
		if err != nil {
			return errors.Wrap(err, "txid invalid")
		}
		if err := statedb.StoreBurningConfirm(bridgeStateDB, *txID, block.GetShardHeader().GetHeight()); err != nil {
			return errors.Wrapf(err, "store failed, txID: %x", txID)
		}
	}
	return nil
}

func updateBridgeIssuanceStatus(bridgeStateDB *statedb.StateDB, block blockinterface.ShardBlockInterface) error {
	for _, tx := range block.GetBody().(blockinterface.ShardBodyInterface).GetTransactions() {
		metaType := tx.GetMetadataType()
		var reqTxID common.Hash
		if metaType == metadata.IssuingETHResponseMeta {
			meta := tx.GetMetadata().(*metadata.IssuingETHResponse)
			reqTxID = meta.RequestedTxID
		} else if metaType == metadata.IssuingResponseMeta {
			meta := tx.GetMetadata().(*metadata.IssuingResponse)
			reqTxID = meta.RequestedTxID
		}
		var err error
		err = statedb.TrackBridgeReqWithStatus(bridgeStateDB, reqTxID, common.BridgeRequestAcceptedStatus)
		if err != nil {
			return err
		}
	}
	return nil
}
