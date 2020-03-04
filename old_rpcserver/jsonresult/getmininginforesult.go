package jsonresult

import (
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/mempool"
)

type GetMiningInfoResult struct {
	ShardHeight         uint64 `json:"ShardHeight"`
	BeaconHeight        uint64 `json:"BeaconHeight"`
	CurrentShardBlockTx int    `json:"CurrentShardBlockTx"`
	PoolSize            int    `json:"PoolSize"`
	Chain               string `json:"Chain"`
	ShardID             int    `json:"ShardID"`
	Layer               string `json:"Layer"`
	Role                string `json:"Role"`
	MiningPublickey     string `json:"MiningPublickey"`
	IsEnableMining      bool   `json:"IsEnableMining"`
}

func NewGetMiningInfoResult(txMemPool mempool.TxPool, blChain blockchain.BlockChain, consensus interface{ GetUserRole() (string, string, int) }, param blockchain.Params, isEnableMining bool) *GetMiningInfoResult {
	result := &GetMiningInfoResult{}
	result.PoolSize = txMemPool.Count()
	result.Chain = param.Name
	result.IsEnableMining = isEnableMining
	result.BeaconHeight = blChain.BestState.Beacon.BeaconHeight

	// role, shardID := httpServer.config.BlockChain.BestState.Beacon.GetPubkeyRole(httpServer.config.MiningPubKeyB58, 0)
	layer, role, shardID := consensus.GetUserRole()
	result.Role = role
	result.Layer = layer
	result.ShardID = shardID
	if shardID >= 0 {
		result.ShardHeight = blChain.BestState.Shard[byte(shardID)].ShardHeight
		result.CurrentShardBlockTx = len(blChain.BestState.Shard[byte(shardID)].BestBlock.Body.Transactions)
	}

	return result
}
