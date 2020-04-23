package rpcserver

import (
	"encoding/json"
	"errors"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
	bnbrelaying "github.com/incognitochain/incognito-chain/relaying/bnb"
	"github.com/incognitochain/incognito-chain/rpcserver/bean"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/rpcserver/rpcservice"
	"github.com/tendermint/tendermint/types"
)

func (httpServer *HttpServer) handleCreateRawTxWithRelayingBTCHeader(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	return httpServer.handleCreateRawTxWithRelayingHeader(
		metadata.RelayingBTCHeaderMeta,
		params,
		closeChan,
	)
}

func (httpServer *HttpServer) handleCreateRawTxWithRelayingBNBHeader(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	return httpServer.handleCreateRawTxWithRelayingHeader(
		metadata.RelayingBNBHeaderMeta,
		params,
		closeChan,
	)
}

func (httpServer *HttpServer) handleCreateRawTxWithRelayingHeader(
	metaType int,
	params interface{},
	closeChan <-chan struct{},
) (interface{}, *rpcservice.RPCError) {
	arrayParams := common.InterfaceSlice(params)
	if len(arrayParams) < 5 {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Param array must be at least 5"))
	}

	// get meta data from params
	data, ok := arrayParams[4].(map[string]interface{})
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("metadata param is invalid"))
	}
	senderAddress, ok := data["SenderAddress"].(string)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("metadata SenderAddress is invalid"))
	}
	// base64encode(marshalbytes), header + lastcommit
	header, ok := data["Header"].(string)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("metadata Header param is invalid"))
	}

	blockHeight, ok := data["BlockHeight"].(float64)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("metadata Header param is invalid"))
	}

	meta, _ := metadata.NewRelayingHeader(
		metaType,
		senderAddress,
		header,
		uint64(blockHeight),
	)

	// create new param to build raw tx from param interface
	createRawTxParam, errNewParam := bean.NewCreateRawTxParam(params)
	if errNewParam != nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errNewParam)
	}
	// HasPrivacyCoin param is always false
	createRawTxParam.HasPrivacyCoin = false

	tx, err1 := httpServer.txService.BuildRawTransaction(createRawTxParam, meta)
	if err1 != nil {
		Logger.log.Error(err1)
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err1)
	}

	byteArrays, err2 := json.Marshal(tx)
	if err2 != nil {
		Logger.log.Error(err1)
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err2)
	}
	result := jsonresult.CreateTransactionResult{
		TxID:            tx.Hash().String(),
		Base58CheckData: base58.Base58Check{}.Encode(byteArrays, 0x00),
	}
	return result, nil
}

func (httpServer *HttpServer) handleCreateAndSendTxWithRelayingBNBHeader(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	data, err := httpServer.handleCreateRawTxWithRelayingBNBHeader(params, closeChan)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err)
	}
	tx := data.(jsonresult.CreateTransactionResult)
	base58CheckData := tx.Base58CheckData
	newParam := make([]interface{}, 0)
	newParam = append(newParam, base58CheckData)
	sendResult, err := httpServer.handleSendRawTransaction(newParam, closeChan)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err)
	}
	result := jsonresult.NewCreateTransactionResult(nil, sendResult.(jsonresult.CreateTransactionResult).TxID, nil, sendResult.(jsonresult.CreateTransactionResult).ShardID)
	return result, nil
}

func (httpServer *HttpServer) handleCreateAndSendTxWithRelayingBTCHeader(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	data, err := httpServer.handleCreateRawTxWithRelayingBTCHeader(params, closeChan)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err)
	}
	tx := data.(jsonresult.CreateTransactionResult)
	base58CheckData := tx.Base58CheckData
	newParam := make([]interface{}, 0)
	newParam = append(newParam, base58CheckData)
	sendResult, err := httpServer.handleSendRawTransaction(newParam, closeChan)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, err)
	}
	result := jsonresult.NewCreateTransactionResult(nil, sendResult.(jsonresult.CreateTransactionResult).TxID, nil, sendResult.(jsonresult.CreateTransactionResult).ShardID)
	return result, nil
}

func (httpServer *HttpServer) handleGetRelayingBNBHeaderState(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	bc := httpServer.config.BlockChain
	relayingState, err := bc.InitRelayingHeaderChainStateFromDB()
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetRelayingBNBHeaderError, err)
	}
	bnbRelayingHeader := relayingState.BNBHeaderChain

	type RelayingBNBHeader struct {
		LatestBlock     *types.Block             `json:"LatestBlock"`
		CandidateBlocks []*types.Block           `json:"CandidateBlocks"`
		OrphanBlocks    map[int64][]*types.Block `json:"OrphanBlocks"`
	}
	result := RelayingBNBHeader{
		LatestBlock:     bnbRelayingHeader.LatestBlock,
		CandidateBlocks: bnbRelayingHeader.CandidateNextBlocks,
		OrphanBlocks:    bnbRelayingHeader.OrphanBlocks,
	}
	return result, nil
}

func (httpServer *HttpServer) handleGetRelayingBNBHeaderByBlockHeight(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	arrayParams := common.InterfaceSlice(params)
	if len(arrayParams) < 1 {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Param array must be at least one"))
	}
	data, ok := arrayParams[0].(map[string]interface{})
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Payload data is invalid"))
	}
	blockHeight, ok := data["BlockHeight"].(float64)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Beacon height is invalid"))
	}

	block, err := httpServer.config.BlockChain.GetBNBBlockByHeight(int64(blockHeight))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetRelayingBNBHeaderByBlockHeightError, err)
	}
	return block, nil
}

func (httpServer *HttpServer) handleGetBTCRelayingBestState(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	bc := httpServer.config.BlockChain
	btcChain := bc.GetConfig().BTCChain
	if btcChain == nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetBTCRelayingBestState, errors.New("BTC relaying chain should not be null"))
	}
	bestState := btcChain.BestSnapshot()
	return bestState, nil
}

func (httpServer *HttpServer) handleGetLatestBNBHeaderBlockHeight(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	bc := httpServer.config.BlockChain
	result, err := bc.GetLatestBNBBlockHeight()
	if err != nil {
		result, _ = bnbrelaying.GetGenesisBNBHeaderBlockHeight(bc.GetConfig().ChainParams.BNBRelayingHeaderChainID)
	}
	return result, nil
}
