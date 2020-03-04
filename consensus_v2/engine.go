package consensus

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/pubsub"
)

var AvailableConsensus map[string]ConsensusInterface

type Engine struct {
	sync.Mutex
	cQuit                chan struct{}
	started              bool
	ChainConsensusList   map[string]ConsensusInterface
	CurrentMiningChain   string
	userMiningPublicKeys map[string]incognitokey.CommitteePublicKey
	userCurrentState     struct {
		UserLayer  string
		UserRole   string
		ShardID    byte
		Keys       *incognitokey.CommitteePublicKey
		KeysBase58 map[string]string
	}
	chainCommitteeChange chan string
	config               *EngineConfig
}

type EngineConfig struct {
	Node       NodeInterface
	Blockchain BlockChainInterface
	// BlockGen      BlockGenInterface
	PubSubManager *pubsub.PubSubManager
}

func New() *Engine {
	return &Engine{}
}

func (engine *Engine) Start() error {
	engine.Lock()
	defer engine.Unlock()
	if engine.started {
		return errors.New("Consensus engine is already started")
	}
	Logger.log.Info("starting consensus...")
	// go engine.config.BlockGen.Start(engine.cQuit)
	go func() {
		go engine.watchConsensusCommittee()
		chainStatus := map[string]bool{}
		for {
			select {
			case <-engine.cQuit:
				return
			default:
				// userKey, _ := engine.GetCurrentMiningPublicKey()
				// metrics.SetGlobalParam("MINING_PUBKEY", userKey)

				time.Sleep(time.Millisecond * 1000)
				for chainName, consensus := range engine.ChainConsensusList {
					if chainName == engine.CurrentMiningChain && engine.userCurrentState.UserRole == common.CommitteeRole {
						if _, ok := chainStatus[chainName]; !ok {
							Logger.log.Critical("BFT: starting bft engine ", chainName)
						}
						consensus.Start()
						if _, ok := chainStatus[chainName]; !ok {
							Logger.log.Critical("BFT: started bft engine ", chainName)
							chainStatus[chainName] = true
						}
					} else {
						if _, ok := chainStatus[chainName]; ok {
							Logger.log.Critical("BFT: stopping bft engine ", chainName)
						}
						consensus.Stop()
						if _, ok := chainStatus[chainName]; ok {
							Logger.log.Critical("BFT: stopped bft engine ", chainName)
							delete(chainStatus, chainName)
						}
					}
				}
			}
		}
	}()
	return nil
}

func (engine *Engine) Stop(name string) error {
	engine.Lock()
	defer engine.Unlock()
	if !engine.started {
		return errors.New("Consensus engine is already stopped")
	}
	engine.started = false
	close(engine.cQuit)
	return nil
}
func RegisterConsensusAlgorithm(name string, consensus ConsensusInterface) error {
	if len(AvailableConsensus) == 0 {
		AvailableConsensus = make(map[string]ConsensusInterface)
	}
	if consensus == nil {
		return NewConsensusError(UnExpectedError, errors.New("consensus can't be nil"))
	}
	AvailableConsensus[name] = consensus
	return nil
}

func (engine *Engine) IsOngoing(chainName string) bool {
	//TO_BE_DELETE
	// consensusModule, ok := engine.ChainConsensusList[chainName]
	// if ok {
	// 	return consensusModule.IsOngoing()
	// }
	// return false
	return true
}

func (engine *Engine) OnBFTMsg(msg ConsensusMsgInterface, sender NodeSender) {
	if engine.CurrentMiningChain == msg.GetChainKey() {
		engine.ChainConsensusList[msg.GetChainKey()].ProcessBFTMsg(msg, sender)
	}
}

func (engine *Engine) GetUserLayer() (string, int) {
	if engine.CurrentMiningChain != "" {
		if engine.userCurrentState.UserLayer == common.BeaconChainKey {
			return engine.userCurrentState.UserLayer, -1
		}
		return engine.userCurrentState.UserLayer, int(engine.userCurrentState.ShardID)
	}
	return "", -2
}

func (engine *Engine) GetUserRole() (string, string, int) {
	//layer,role,shardID
	if engine.CurrentMiningChain != "" {
		userLayer := engine.userCurrentState.UserLayer
		userRole := engine.userCurrentState.UserRole
		shardID := -1
		if userRole == common.CommitteeRole || userRole == common.PendingRole {
			if userLayer == common.ShardRole {
				shardID = int(engine.userCurrentState.ShardID)
			}
		}
		return engine.userCurrentState.UserLayer, engine.userCurrentState.UserRole, shardID
	}
	return "", "", -2
}

func getShardFromChainName(chainName string) byte {
	s := strings.Split(chainName, "-")[1]
	s1, _ := strconv.Atoi(s)
	return byte(s1)
}

func (engine *Engine) NotifyBeaconRole(beaconRole bool) {
	engine.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.BeaconRoleTopic, beaconRole))
}
func (engine *Engine) NotifyShardRole(shardRole int) {
	engine.config.PubSubManager.PublishMessage(pubsub.NewMessage(pubsub.ShardRoleTopic, shardRole))
}

func (engine *Engine) Init(config *EngineConfig) error {
	// if config.BlockGen == nil {
	// 	return NewConsensusError(UnExpectedError, errors.New("BlockGen can't be nil"))
	// }
	if config.Blockchain == nil {
		return NewConsensusError(UnExpectedError, errors.New("Blockchain can't be nil"))
	}
	if config.Node == nil {
		return NewConsensusError(UnExpectedError, errors.New("Node can't be nil"))
	}
	if config.PubSubManager == nil {
		return NewConsensusError(UnExpectedError, errors.New("PubSubManager can't be nil"))
	}
	engine.config = config
	engine.cQuit = make(chan struct{})
	engine.chainCommitteeChange = make(chan string)
	engine.ChainConsensusList = make(map[string]ConsensusInterface)
	if config.Node.GetPrivateKey() != "" {
		keyList, err := engine.GenMiningKeyFromPrivateKey(config.Node.GetPrivateKey())
		if err != nil {
			panic(err)
		}
		err = engine.LoadMiningKeys(keyList)
		if err != nil {
			panic(err)
		}
	} else if config.Node.GetMiningKeys() != "" {
		err := engine.LoadMiningKeys(config.Node.GetMiningKeys())
		if err != nil {
			panic(err)
		}
	}
	return nil
}

func (engine *Engine) ExtractBridgeValidationData(block blockinterface.BlockInterface) ([][]byte, []int, error) {
	if _, ok := AvailableConsensus[block.GetHeader().GetConsensusType()]; ok {
		return AvailableConsensus[block.GetHeader().GetConsensusType()].ExtractBridgeValidationData(block)
	}
	return nil, nil, NewConsensusError(ConsensusTypeNotExistError, errors.New(block.GetHeader().GetConsensusType()))
}

func (engine *Engine) updateConsensusState() {
	userLayer := ""
	if engine.CurrentMiningChain == common.BeaconChainKey {
		userLayer = common.BeaconRole
		go engine.NotifyBeaconRole(true)
		go engine.NotifyShardRole(-1)
	}
	if engine.CurrentMiningChain != common.BeaconChainKey && engine.CurrentMiningChain != "" {
		userLayer = common.ShardRole
		go engine.NotifyBeaconRole(false)
		go engine.NotifyShardRole(int(getShardFromChainName(engine.CurrentMiningChain)))
	}
	publicKey, err := engine.GetMiningPublicKeyByConsensus(engine.config.Blockchain.GetChain(common.BeaconChainKey).GetBestView().GetConsensusType())
	if err != nil {
		Logger.log.Error(err)
		return
	}
	//ExtractMiningPublickeysFromCommitteeKeyList
	allcommittee := engine.config.Blockchain.GetChain(common.BeaconChainKey).(BeaconManagerInterface).GetAllCommittees()
	beaconCommittee := []string{}
	shardCommittee := map[byte][]string{}
	shardCommittee = make(map[byte][]string)

	for keyType, committees := range allcommittee {
		for chain, committee := range committees {
			if chain == common.BeaconChainKey {
				keyList, _ := incognitokey.ExtractMiningPublickeysFromCommitteeKeyList(committee, keyType)
				beaconCommittee = append(beaconCommittee, keyList...)
			} else {
				keyList, _ := incognitokey.ExtractMiningPublickeysFromCommitteeKeyList(committee, keyType)
				shardCommittee[getShardFromChainName(chain)] = keyList
			}
		}
	}
	if userLayer == common.ShardRole {
		committee := engine.config.Blockchain.GetChain(engine.CurrentMiningChain).GetBestView().GetCommittee()
		keyList, _ := incognitokey.ExtractMiningPublickeysFromCommitteeKeyList(committee, engine.config.Blockchain.GetChain(engine.CurrentMiningChain).GetBestView().GetConsensusType())
		shardCommittee[getShardFromChainName(engine.CurrentMiningChain)] = keyList
	}

	fmt.Printf("UpdateConsensusState %v %v\n", userLayer, publicKey)
	if userLayer == common.ShardRole {
		shardID := getShardFromChainName(engine.CurrentMiningChain)
		engine.config.Node.UpdateConsensusState(userLayer, publicKey, &shardID, beaconCommittee, shardCommittee)
	} else {
		engine.config.Node.UpdateConsensusState(userLayer, publicKey, nil, beaconCommittee, shardCommittee)
	}
}

func (engine *Engine) updateUserState(keySet *incognitokey.CommitteePublicKey, layer string, role string, shardID byte) {
	isChange := false

	if engine.userCurrentState.ShardID != shardID {
		isChange = true
	}
	if engine.userCurrentState.UserLayer != layer {
		isChange = true
	}
	if engine.userCurrentState.Keys != nil {
		incKey, ok := engine.userCurrentState.KeysBase58[common.IncKeyType]
		if ok {
			if incKey != keySet.GetIncKeyBase58() {
				isChange = true
			}
		}
	}

	if role == "" {
		// metrics.SetGlobalParam("Layer", "", "Role", "", "ShardID", -2)
		engine.userCurrentState.UserLayer = ""
		engine.userCurrentState.UserRole = ""
		engine.userCurrentState.ShardID = 0
		engine.userCurrentState.Keys = nil
		engine.userCurrentState.KeysBase58 = make(map[string]string)
	} else {
		// metrics.SetGlobalParam("Layer", layer, "Role", role, "ShardID", shardID)
		engine.userCurrentState.ShardID = shardID
		engine.userCurrentState.UserLayer = layer
		engine.userCurrentState.UserRole = role
		engine.userCurrentState.Keys = keySet
		engine.userCurrentState.KeysBase58 = make(map[string]string)
		engine.userCurrentState.KeysBase58[common.IncKeyType] = keySet.GetIncKeyBase58()
		for keyType := range keySet.MiningPubKey {
			engine.userCurrentState.KeysBase58[keyType] = keySet.GetMiningKeyBase58(keyType)
		}
	}

	engine.updateConsensusState()

	if isChange {
		engine.config.Node.DropAllConnections()
	}
}

func (engine *Engine) GetMiningPublicKeys() map[string]incognitokey.CommitteePublicKey {
	return engine.userMiningPublicKeys
}
