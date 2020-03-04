package consensus

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

func (engine *Engine) CommitteeChange(chainName string) {
	engine.chainCommitteeChange <- chainName
}

//watchConsensusState will watch MiningKey Role as well as chain consensus type
func (engine *Engine) watchConsensusCommittee() {
	Logger.log.Info("start watching consensus committee...")
	allcommittee := engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetAllCommittees()

	for consensusType, publickey := range engine.userMiningPublicKeys {
		if committees, ok := allcommittee[consensusType]; ok {
			for chainName, committee := range committees {
				keys, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(committee, consensusType)
				if common.IndexOfStr(publickey.GetMiningKeyBase58(consensusType), keys) != -1 {
					engine.CurrentMiningChain = chainName
					var userRole, userLayer string
					var shardID byte
					if chainName != common.BeaconChainKey {
						userLayer = common.ShardRole
						userRole = common.CommitteeRole
						shardID = getShardFromChainName(chainName)
					} else {
						userLayer = common.BeaconRole
						userRole = common.CommitteeRole
					}
					engine.updateUserState(&publickey, userLayer, userRole, shardID)
					break
				}
			}
		}
	}

	if engine.CurrentMiningChain == "" {

		shardsPendingLists := engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetShardsPendingList()

		for consensusType, publickey := range engine.userMiningPublicKeys {
			beaconPendingList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetBeaconPendingList(), consensusType)
			beaconWaitingList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetBeaconWaitingList(), consensusType)
			shardsWaitingList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetShardsWaitingList(), consensusType)

			var shardsPendingList map[string][]string
			shardsPendingList = make(map[string][]string)

			for chainName, committee := range shardsPendingLists[consensusType] {
				shardsPendingList[chainName], _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(committee, consensusType)
			}

			if common.IndexOfStr(publickey.GetMiningKeyBase58(consensusType), beaconPendingList) != -1 {
				engine.CurrentMiningChain = common.BeaconChainKey
				engine.updateUserState(&publickey, common.BeaconRole, common.PendingRole, 0)
				break
			}
			if common.IndexOfStr(publickey.GetMiningKeyBase58(consensusType), beaconWaitingList) != -1 {
				engine.CurrentMiningChain = common.BeaconChainKey
				engine.updateUserState(&publickey, common.BeaconRole, common.WaitingRole, 0)
				break
			}
			if common.IndexOfStr(publickey.GetMiningKeyBase58(consensusType), shardsWaitingList) != -1 {
				engine.CurrentMiningChain = common.BeaconChainKey
				engine.updateUserState(&publickey, common.ShardRole, common.WaitingRole, 0)
				break
			}
			for chainName, committee := range shardsPendingList {
				if common.IndexOfStr(publickey.GetMiningKeyBase58(consensusType), committee) != -1 {
					engine.CurrentMiningChain = chainName
					shardID := getShardFromChainName(chainName)
					if engine.config.Blockchain.GetChain(common.GetShardChainKey(shardID)).GetBestView().GetHeight() > engine.config.Blockchain.GetChain(common.BeaconChainKey).GetBestView().(BeaconViewInterface).GetBestHeightOfShard(shardID) {
						role, shardID := engine.config.Blockchain.GetChain(chainName).GetBestView().GetPubkeyRole(publickey.GetMiningKeyBase58(consensusType), 0)
						if role == common.ProposerRole || role == common.ValidatorRole {
							engine.updateUserState(&publickey, common.ShardRole, common.CommitteeRole, shardID)
						} else {
							if role == common.PendingRole {
								engine.updateUserState(&publickey, common.ShardRole, common.PendingRole, getShardFromChainName(chainName))
							}
						}
						break
					}
					engine.updateUserState(&publickey, common.ShardRole, common.PendingRole, getShardFromChainName(chainName))
				}
			}
			if engine.CurrentMiningChain != "" {
				break
			}
		}

	}

	for chainName, chain := range engine.config.Blockchain.GetAllChains() {
		if _, ok := AvailableConsensus[chain.GetBestView().GetConsensusType()]; ok {
			engine.ChainConsensusList[chainName] = AvailableConsensus[chain.GetBestView().GetConsensusType()].NewInstance(chain, chainName, engine.config.Node, Logger.log)
		}
	}

	if engine.CurrentMiningChain == common.BeaconChainKey {
		go engine.NotifyBeaconRole(true)
		go engine.NotifyShardRole(-1)
	}
	if engine.CurrentMiningChain != common.BeaconChainKey && engine.CurrentMiningChain != "" {
		go engine.NotifyBeaconRole(false)
		go engine.NotifyShardRole(int(getShardFromChainName(engine.CurrentMiningChain)))
	}

	for {
		select {
		case <-engine.cQuit:
		case chainName := <-engine.chainCommitteeChange:
			Logger.log.Critical("chain committee change", chainName)
			consensusType := engine.config.Blockchain.GetChain(chainName).GetBestView().GetConsensusType()
			userCurrentPublicKey, ok := engine.userCurrentState.KeysBase58[consensusType]
			var userMiningKey incognitokey.CommitteePublicKey
			if !ok {
				userMiningKey, ok = engine.userMiningPublicKeys[consensusType]
				if !ok {
					continue
				}
				userCurrentPublicKey = userMiningKey.GetMiningKeyBase58(consensusType)
			} else {
				userMiningKey = engine.userMiningPublicKeys[consensusType]
			}

			if chainName == common.BeaconChainKey || engine.userCurrentState.UserRole == common.WaitingRole {
				allcommittee := engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetAllCommittees()
				isSkip := false
				if committees, ok := allcommittee[consensusType]; ok {
					for chainname, committee := range committees {
						keys, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(committee, consensusType)
						if common.IndexOfStr(userCurrentPublicKey, keys) != -1 {
							engine.CurrentMiningChain = chainname
							var userRole, userLayer string
							var shardID byte
							if chainname != common.BeaconChainKey {
								shardID = getShardFromChainName(chainname)
								userLayer = common.ShardRole
								//member still in shard committee on beacon beststate but not on shard beststate
								if engine.config.Blockchain.GetChain(common.GetShardChainKey(shardID)).GetBestView().GetHeight() > engine.config.Blockchain.GetChain(common.BeaconChainKey).GetBestView().(BeaconViewInterface).GetBestHeightOfShard(shardID) {
									role, _ := engine.config.Blockchain.GetChain(chainname).GetBestView().GetPubkeyRole(userCurrentPublicKey, 0)
									if role == common.EmptyString {
										isSkip = true
										engine.CurrentMiningChain = common.EmptyString
										engine.updateUserState(&userMiningKey, common.EmptyString, common.EmptyString, 0)
										break
									}
								}
							} else {
								isSkip = true
								userLayer = common.BeaconRole
							}

							userRole = common.CommitteeRole
							engine.updateUserState(&userMiningKey, userLayer, userRole, shardID)
							break
						} else {
							if chainname == engine.CurrentMiningChain && chainname != common.BeaconChainKey {
								shardID := getShardFromChainName(chainname)
								if engine.config.Blockchain.GetChain(common.GetShardChainKey(shardID)).GetBestView().GetHeight() > engine.config.Blockchain.GetChain(common.BeaconChainKey).GetBestView().(BeaconViewInterface).GetBestHeightOfShard(shardID) {
									role, _ := engine.config.Blockchain.GetChain(chainname).GetBestView().GetPubkeyRole(userCurrentPublicKey, 0)
									if role == common.ValidatorRole || role == common.ProposerRole {
										isSkip = true
										engine.updateUserState(&userMiningKey, common.ShardRole, common.CommitteeRole, shardID)
										break
									}
								} else {
									engine.CurrentMiningChain = common.EmptyString
									engine.updateUserState(&userMiningKey, common.EmptyString, common.EmptyString, 0)
									break
								}
							}
						}
					}
				}

				if isSkip {
					continue
				}

				if engine.CurrentMiningChain == common.EmptyString || engine.userCurrentState.UserRole == common.WaitingRole {
					shardsPendingLists := engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetShardsPendingList()
					beaconPendingList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetBeaconPendingList(), consensusType)
					shardsWaitingList, _ := incognitokey.ExtractPublickeysFromCommitteeKeyList(engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetShardsWaitingList(), consensusType)

					var shardsPendingList map[string][]string
					shardsPendingList = make(map[string][]string)

					for chainName, committee := range shardsPendingLists[consensusType] {
						shardsPendingList[chainName], _ = incognitokey.ExtractPublickeysFromCommitteeKeyList(committee, consensusType)
					}

					if common.IndexOfStr(userCurrentPublicKey, beaconPendingList) != -1 {
						engine.CurrentMiningChain = common.BeaconChainKey
						engine.updateUserState(&userMiningKey, common.BeaconRole, common.PendingRole, 0)
						continue
					}
					if common.IndexOfStr(userCurrentPublicKey, shardsWaitingList) != -1 {
						engine.CurrentMiningChain = common.BeaconChainKey
						engine.updateUserState(&userMiningKey, common.ShardRole, common.WaitingRole, 0)
						continue
					}
					for chainName, committee := range shardsPendingList {
						if common.IndexOfStr(userCurrentPublicKey, committee) != -1 {
							engine.CurrentMiningChain = chainName
							engine.updateUserState(&userMiningKey, common.ShardRole, common.PendingRole, getShardFromChainName(chainName))
							break
						}
					}
					if engine.CurrentMiningChain != "" {
						continue
					}
				}

				if engine.userCurrentState.UserLayer == common.BeaconRole {
					engine.CurrentMiningChain = common.EmptyString
					engine.updateUserState(&userMiningKey, common.EmptyString, common.EmptyString, 0)
				}
			} else {
				role, shardID := engine.config.Blockchain.GetChain(chainName).GetBestView().GetPubkeyRole(userCurrentPublicKey, 0)
				if role != common.EmptyString {
					if role == common.ValidatorRole || role == common.ProposerRole {
						role = common.CommitteeRole
						engine.updateUserState(&userMiningKey, common.ShardRole, role, shardID)
					}
				} else {
					if engine.CurrentMiningChain == chainName {
						shardID := getShardFromChainName(chainName)
						if engine.config.Blockchain.GetChain(common.GetShardChainKey(shardID)).GetBestView().GetHeight() > engine.config.Blockchain.GetChain(common.BeaconChainKey).GetBestView().(BeaconViewInterface).GetBestHeightOfShard(shardID) {
							engine.CurrentMiningChain = common.EmptyString
							engine.updateUserState(&userMiningKey, common.EmptyString, common.EmptyString, 0)
						}
					}
				}
			}
		}
	}
}
