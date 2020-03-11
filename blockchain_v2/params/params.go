package params

import (
	"time"

	"github.com/incognitochain/incognito-chain/blockchain_v2/app"
	"github.com/incognitochain/incognito-chain/blockchain_v2/types/blockinterface"
	"github.com/incognitochain/incognito-chain/common"
)

/*
Params defines a network by its component. These component may be used by Applications
to differentiate network as well as addresses and keys for one network
from those intended for use on another network
*/
type Params struct {
	Name                             string // Name defines a human-readable identifier for the network.
	Net                              uint32 // Net defines the magic bytes used to identify the network.
	DefaultPort                      string // DefaultPort defines the default peer-to-peer port for the network.
	MaxShardCommitteeSize            int
	MinShardCommitteeSize            int
	MaxBeaconCommitteeSize           int
	MinBeaconCommitteeSize           int
	MinShardBlockInterval            time.Duration
	MaxShardBlockCreation            time.Duration
	MinBeaconBlockInterval           time.Duration
	MaxBeaconBlockCreation           time.Duration
	StakingAmountShard               uint64
	ActiveShards                     int
	GenesisBeaconBlock               blockinterface.BeaconBlockInterface // GenesisBlock defines the first block of the chain.
	GenesisShardBlock                blockinterface.ShardBlockInterface  // GenesisBlock defines the first block of the chain.
	BasicReward                      uint64
	Epoch                            uint64
	RandomTime                       uint64
	SlashLevels                      []SlashLevel
	EthContractAddressStr            string // smart contract of ETH for bridge
	Offset                           int    // default offset for swap policy, is used for cases that good producers length is less than max committee size
	SwapOffset                       int    // is used for case that good producers length is equal to max committee size
	IncognitoDAOAddress              string
	CentralizedWebsitePaymentAddress string //centralized website's pubkey
	CheckForce                       bool   // true on testnet and false on mainnet
	ChainVersion                     string
	AssignOffset                     int
	BeaconHeightBreakPointBurnAddr   uint64
}

type SlashLevel struct {
	MinRange        uint8
	PunishedEpoches uint8
}

type GenesisParams struct {
	InitialIncognito                            []string // init tx for genesis block
	FeePerTxKb                                  uint64
	PreSelectBeaconNodeSerializedPubkey         []string
	PreSelectBeaconNodeSerializedPaymentAddress []string
	PreSelectBeaconNode                         []string
	PreSelectShardNodeSerializedPubkey          []string
	PreSelectShardNodeSerializedPaymentAddress  []string
	PreSelectShardNode                          []string
	ConsensusAlgorithm                          string
}

var ChainTestParam = Params{}
var ChainMainParam = Params{}

// FOR TESTNET
func init() {
	var genesisParamsTestnetNew = GenesisParams{
		PreSelectBeaconNodeSerializedPubkey:         PreSelectBeaconNodeTestnetSerializedPubkey,
		PreSelectBeaconNodeSerializedPaymentAddress: PreSelectBeaconNodeTestnetSerializedPaymentAddress,
		PreSelectShardNodeSerializedPubkey:          PreSelectShardNodeTestnetSerializedPubkey,
		PreSelectShardNodeSerializedPaymentAddress:  PreSelectShardNodeTestnetSerializedPaymentAddress,
		//@Notice: InitTxsForBenchmark is for testing and testparams only
		//InitialIncognito: IntegrationTestInitPRV,
		InitialIncognito:   TestnetInitPRV,
		ConsensusAlgorithm: common.BlsConsensus,
	}
	ChainTestParam = Params{
		Name:                   TestnetName,
		Net:                    Testnet,
		DefaultPort:            TestnetDefaultPort,
		MaxShardCommitteeSize:  TestNetShardCommitteeSize,     //TestNetShardCommitteeSize,
		MinShardCommitteeSize:  TestNetMinShardCommitteeSize,  //TestNetShardCommitteeSize,
		MaxBeaconCommitteeSize: TestNetBeaconCommitteeSize,    //TestNetBeaconCommitteeSize,
		MinBeaconCommitteeSize: TestNetMinBeaconCommitteeSize, //TestNetBeaconCommitteeSize,
		StakingAmountShard:     TestNetStakingAmountShard,
		ActiveShards:           TestNetActiveShards,
		// blockChain parameters
		GenesisBeaconBlock: app.CreateBeaconGenesisBlock(1, Testnet, TestnetGenesisBlockTime,
			GenesisParamsTestnetNew.PreSelectBeaconNodeSerializedPubkey,
			GenesisParamsTestnetNew.PreSelectBeaconNodeSerializedPaymentAddress,
			GenesisParamsTestnetNew.PreSelectShardNodeSerializedPubkey,
			GenesisParamsTestnetNew.PreSelectShardNodeSerializedPaymentAddress),
		GenesisShardBlock:                app.CreateShardGenesisBlock(1, Testnet, TestnetGenesisBlockTime, TestnetInitPRV),
		MinShardBlockInterval:            TestNetMinShardBlkInterval,
		MaxShardBlockCreation:            TestNetMaxShardBlkCreation,
		MinBeaconBlockInterval:           TestNetMinBeaconBlkInterval,
		MaxBeaconBlockCreation:           TestNetMaxBeaconBlkCreation,
		BasicReward:                      TestnetBasicReward,
		Epoch:                            TestnetEpoch,
		RandomTime:                       TestnetRandomTime,
		Offset:                           TestnetOffset,
		AssignOffset:                     TestnetAssignOffset,
		SwapOffset:                       TestnetSwapOffset,
		EthContractAddressStr:            TestnetETHContractAddressStr,
		IncognitoDAOAddress:              TestnetIncognitoDAOAddress,
		CentralizedWebsitePaymentAddress: TestnetCentralizedWebsitePaymentAddress,
		SlashLevels: []SlashLevel{
			//SlashLevel{MinRange: 20, PunishedEpoches: 1},
			SlashLevel{MinRange: 50, PunishedEpoches: 2},
			SlashLevel{MinRange: 75, PunishedEpoches: 3},
		},
		CheckForce:                     false,
		ChainVersion:                   "version-chain-test.json",
		BeaconHeightBreakPointBurnAddr: 250000,
	}
	// END TESTNET
	// FOR MAINNET
	var genesisParamsMainnetNew = GenesisParams{
		PreSelectBeaconNodeSerializedPubkey:         PreSelectBeaconNodeMainnetSerializedPubkey,
		PreSelectBeaconNodeSerializedPaymentAddress: PreSelectBeaconNodeMainnetSerializedPaymentAddress,
		PreSelectShardNodeSerializedPubkey:          PreSelectShardNodeMainnetSerializedPubkey,
		PreSelectShardNodeSerializedPaymentAddress:  PreSelectShardNodeMainnetSerializedPaymentAddress,
		InitialIncognito:                            MainnetInitPRV,
		ConsensusAlgorithm:                          common.BlsConsensus,
	}
	ChainMainParam = Params{
		Name:                   MainetName,
		Net:                    Mainnet,
		DefaultPort:            MainnetDefaultPort,
		MaxShardCommitteeSize:  MainNetShardCommitteeSize, //MainNetShardCommitteeSize,
		MinShardCommitteeSize:  MainNetMinShardCommitteeSize,
		MaxBeaconCommitteeSize: MainNetBeaconCommitteeSize, //MainNetBeaconCommitteeSize,
		MinBeaconCommitteeSize: MainNetMinBeaconCommitteeSize,
		StakingAmountShard:     MainNetStakingAmountShard,
		ActiveShards:           MainNetActiveShards,
		// blockChain parameters
		GenesisBeaconBlock: app.CreateBeaconGenesisBlock(1, Mainnet, MainnetGenesisBlockTime,
			genesisParamsMainnetNew.PreSelectBeaconNodeSerializedPubkey,
			genesisParamsMainnetNew.PreSelectBeaconNodeSerializedPaymentAddress,
			genesisParamsMainnetNew.PreSelectShardNodeSerializedPubkey,
			genesisParamsMainnetNew.PreSelectShardNodeSerializedPaymentAddress),
		GenesisShardBlock:                app.CreateShardGenesisBlock(1, Mainnet, MainnetGenesisBlockTime, MainnetInitPRV),
		MinShardBlockInterval:            MainnetMinShardBlkInterval,
		MaxShardBlockCreation:            MainnetMaxShardBlkCreation,
		MinBeaconBlockInterval:           MainnetMinBeaconBlkInterval,
		MaxBeaconBlockCreation:           MainnetMaxBeaconBlkCreation,
		BasicReward:                      MainnetBasicReward,
		Epoch:                            MainnetEpoch,
		RandomTime:                       MainnetRandomTime,
		Offset:                           MainnetOffset,
		SwapOffset:                       MainnetSwapOffset,
		AssignOffset:                     MainnetAssignOffset,
		EthContractAddressStr:            MainETHContractAddressStr,
		IncognitoDAOAddress:              MainnetIncognitoDAOAddress,
		CentralizedWebsitePaymentAddress: MainnetCentralizedWebsitePaymentAddress,
		SlashLevels: []SlashLevel{
			//SlashLevel{MinRange: 20, PunishedEpoches: 1},
			SlashLevel{MinRange: 50, PunishedEpoches: 2},
			SlashLevel{MinRange: 75, PunishedEpoches: 3},
		},
		CheckForce:                     false,
		ChainVersion:                   "version-chain-main.json",
		BeaconHeightBreakPointBurnAddr: 150500,
	}
}
