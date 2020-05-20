package common

// for common
const (
	EmptyString       = ""
	ZeroByte          = byte(0x00)
	DateOutputFormat  = "2006-01-02T15:04:05.999999"
	BigIntSize        = 32 // bytes
	CheckSumLen       = 4  // bytes
	AESKeySize        = 32 // bytes
	Int32Size         = 4  // bytes
	Uint32Size        = 4  // bytes
	Uint64Size        = 8  // bytes
	HashSize          = 32 // bytes
	MaxHashStringSize = HashSize * 2
	Base58Version     = 0
)

// size data for incognito key and signature
const (
	// for key size
	PrivateKeySize      = 32  // bytes
	PublicKeySize       = 32  // bytes
	BLSPublicKeySize    = 128 // bytes
	BriPublicKeySize    = 33  // bytes
	TransmissionKeySize = 32  //bytes
	ReceivingKeySize    = 32  // bytes
	PaymentAddressSize  = 64  // bytes
	// for signature size
	// it is used for both privacy and no privacy
	SigPubKeySize    = 32
	SigNoPrivacySize = 64
	SigPrivacySize   = 96
	IncPubKeyB58Size = 51
)

// for exit code
const (
	ExitCodeUnknow = iota
	ExitByOs
	ExitByLogging
	ExitCodeForceUpdate
)

// For all Transaction information
const (
	TxNormalType        = "n"  // normal tx(send and receive coin)
	TxRewardType        = "s"  // reward tx
	TxReturnStakingType = "rs" //
	//TxCustomTokenType        = "t"  // token  tx with no supporting privacy
	TxCustomTokenPrivacyType = "tp" // token  tx with supporting privacy
)

var (
	MaxTxSize    = uint64(100)  // unit KB = 100KB
	MaxBlockSize = uint64(2000) //unit kilobytes = 2 Megabyte
)

// special token ids (aka. PropertyID in custom token)
var (
	PRVCoinID   = Hash{4} // To send PRV in custom token
	PRVCoinName = "PRV"   // To send PRV in custom token
)

// CONSENSUS
const (
	NodeModeRelay  = "relay"
	NodeModeShard  = "shard"
	NodeModeAuto   = "auto"
	NodeModeBeacon = "beacon"

	BeaconRole     = "beacon"
	ShardRole      = "shard"
	CommitteeRole  = "committee"
	ProposerRole   = "proposer"
	ValidatorRole  = "validator"
	PendingRole    = "pending"
	SyncingRole    = "syncing" //this is for shard case - when beacon tell it is committee, but its state not
	WaitingRole    = "waiting"
	MaxShardNumber = 8

	BlsConsensus    = "bls"
	BridgeConsensus = "dsa"
	IncKeyType      = "inc"
)

const (
	BeaconChainKey = "beacon"
	ShardChainKey  = "shard"
)

const (
	BeaconChainDataBaseID        = -1
	BeaconChainDatabaseDirectory = "beacon"
	ShardChainDatabaseDirectory  = "shard"
)

// Ethereum Decentralized bridge
const (
	//AbiJson       = `[{"inputs":[{"internalType":"address","name":"admin","type":"address"},{"internalType":"address","name":"incognitoProxyAddress","type":"address"},{"internalType":"address","name":"_prevVault","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"claimer","type":"address"}],"name":"Claim","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"string","name":"incognitoAddress","type":"string"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"Deposit","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"ndays","type":"uint256"}],"name":"Extend","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"newVault","type":"address"}],"name":"Migrate","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address[]","name":"assets","type":"address[]"}],"name":"MoveAssets","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"pauser","type":"address"}],"name":"Paused","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"pauser","type":"address"}],"name":"Unpaused","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"newIncognitoProxy","type":"address"}],"name":"UpdateIncognitoProxy","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"Withdraw","type":"event"},{"payable":true,"stateMutability":"payable","type":"fallback"},{"constant":true,"inputs":[],"name":"ETH_TOKEN","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"admin","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"claim","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"string","name":"incognitoAddress","type":"string"}],"name":"deposit","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"incognitoAddress","type":"string"}],"name":"depositERC20","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":true,"inputs":[],"name":"expire","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"uint256","name":"n","type":"uint256"}],"name":"extend","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"internalType":"address","name":"token","type":"address"}],"name":"getDecimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"incognito","outputs":[{"internalType":"contractIncognito","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"internalType":"bytes32","name":"hash","type":"bytes32"}],"name":"isWithdrawed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"addresspayable","name":"_newVault","type":"address"}],"name":"migrate","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"address[]","name":"assets","type":"address[]"}],"name":"moveAssets","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"newVault","outputs":[{"internalType":"addresspayable","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[{"internalType":"bytes","name":"inst","type":"bytes"}],"name":"parseBurnInst","outputs":[{"internalType":"uint8","name":"","type":"uint8"},{"internalType":"uint8","name":"","type":"uint8"},{"internalType":"address","name":"","type":"address"},{"internalType":"addresspayable","name":"","type":"address"},{"internalType":"uint256","name":"","type":"uint256"}],"payable":false,"stateMutability":"pure","type":"function"},{"constant":false,"inputs":[],"name":"pause","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"paused","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"prevVault","outputs":[{"internalType":"contractWithdrawable","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"_successor","type":"address"}],"name":"retire","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[],"name":"successor","outputs":[{"internalType":"address","name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"unpause","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"address","name":"newIncognitoProxy","type":"address"}],"name":"updateIncognitoProxy","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"internalType":"bytes","name":"inst","type":"bytes"},{"internalType":"uint256[2]","name":"heights","type":"uint256[2]"},{"internalType":"bytes32[][2]","name":"instPaths","type":"bytes32[][2]"},{"internalType":"bool[][2]","name":"instPathIsLefts","type":"bool[][2]"},{"internalType":"bytes32[2]","name":"instRoots","type":"bytes32[2]"},{"internalType":"bytes32[2]","name":"blkData","type":"bytes32[2]"},{"internalType":"uint256[][2]","name":"sigIdxs","type":"uint256[][2]"},{"internalType":"uint8[][2]","name":"sigVs","type":"uint8[][2]"},{"internalType":"bytes32[][2]","name":"sigRs","type":"bytes32[][2]"},{"internalType":"bytes32[][2]","name":"sigSs","type":"bytes32[][2]"}],"name":"withdraw","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"withdrawed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"payable":false,"stateMutability":"view","type":"function"}]`
	AbiJson       = `[{"inputs":[{"internalType":"address","name":"admin","type":"address"},{"internalType":"address","name":"incognitoProxyAddress","type":"address"},{"internalType":"address","name":"_prevVault","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"claimer","type":"address"}],"name":"Claim","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"string","name":"incognitoAddress","type":"string"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"Deposit","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"ndays","type":"uint256"}],"name":"Extend","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"newVault","type":"address"}],"name":"Migrate","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address[]","name":"assets","type":"address[]"}],"name":"MoveAssets","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"pauser","type":"address"}],"name":"Paused","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"pauser","type":"address"}],"name":"Unpaused","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"newIncognitoProxy","type":"address"}],"name":"UpdateIncognitoProxy","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address[]","name":"assets","type":"address[]"},{"indexed":false,"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"name":"UpdateTokenTotal","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"token","type":"address"},{"indexed":false,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"Withdraw","type":"event"},{"inputs":[],"name":"ETH_TOKEN","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"admin","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"token","type":"address"}],"name":"balanceOf","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"claim","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string","name":"incognitoAddress","type":"string"}],"name":"deposit","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"string","name":"incognitoAddress","type":"string"}],"name":"depositERC20","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"recipientToken","type":"address"},{"internalType":"address","name":"exchangeAddress","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"},{"internalType":"bytes","name":"timestamp","type":"bytes"},{"internalType":"bytes","name":"signData","type":"bytes"}],"name":"execute","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[{"internalType":"address[]","name":"tokens","type":"address[]"},{"internalType":"uint256[]","name":"amounts","type":"uint256[]"},{"internalType":"address[]","name":"recipientTokens","type":"address[]"},{"internalType":"address","name":"exchangeAddress","type":"address"},{"internalType":"bytes","name":"callData","type":"bytes"},{"internalType":"bytes","name":"timestamp","type":"bytes"},{"internalType":"bytes","name":"signData","type":"bytes"}],"name":"executeMulti","outputs":[],"stateMutability":"payable","type":"function"},{"inputs":[],"name":"expire","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"n","type":"uint256"}],"name":"extend","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"token","type":"address"}],"name":"getDecimals","outputs":[{"internalType":"uint8","name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"address","name":"owner","type":"address"}],"name":"getDepositedBalance","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"incognito","outputs":[{"internalType":"contract Incognito","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"hash","type":"bytes32"}],"name":"isSigDataUsed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"hash","type":"bytes32"}],"name":"isWithdrawed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address payable","name":"_newVault","type":"address"}],"name":"migrate","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address[]","name":"assets","type":"address[]"}],"name":"moveAssets","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"newVault","outputs":[{"internalType":"address payable","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"notEntered","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes","name":"inst","type":"bytes"}],"name":"parseBurnInst","outputs":[{"internalType":"uint8","name":"","type":"uint8"},{"internalType":"uint8","name":"","type":"uint8"},{"internalType":"address","name":"","type":"address"},{"internalType":"address payable","name":"","type":"address"},{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"pause","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"paused","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"prevVault","outputs":[{"internalType":"contract Withdrawable","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"incognitoAddress","type":"string"},{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"bytes","name":"signData","type":"bytes"},{"internalType":"bytes","name":"timestamp","type":"bytes"}],"name":"requestWithdraw","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"_successor","type":"address"}],"name":"retire","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"sigDataUsed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes","name":"signData","type":"bytes"},{"internalType":"bytes32","name":"hash","type":"bytes32"}],"name":"sigToAddress","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"pure","type":"function"},{"inputs":[{"internalType":"bytes","name":"inst","type":"bytes"},{"internalType":"uint256[2]","name":"heights","type":"uint256[2]"},{"internalType":"bytes32[][2]","name":"instPaths","type":"bytes32[][2]"},{"internalType":"bool[][2]","name":"instPathIsLefts","type":"bool[][2]"},{"internalType":"bytes32[2]","name":"instRoots","type":"bytes32[2]"},{"internalType":"bytes32[2]","name":"blkData","type":"bytes32[2]"},{"internalType":"uint256[][2]","name":"sigIdxs","type":"uint256[][2]"},{"internalType":"uint8[][2]","name":"sigVs","type":"uint8[][2]"},{"internalType":"bytes32[][2]","name":"sigRs","type":"bytes32[][2]"},{"internalType":"bytes32[][2]","name":"sigSs","type":"bytes32[][2]"}],"name":"submitBurnProof","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"successor","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"}],"name":"totalDepositedToSCAmount","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"unpause","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address[]","name":"assets","type":"address[]"},{"internalType":"uint256[]","name":"amounts","type":"uint256[]"}],"name":"updateAssets","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"newIncognitoProxy","type":"address"}],"name":"updateIncognitoProxy","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes","name":"inst","type":"bytes"},{"internalType":"uint256[2]","name":"heights","type":"uint256[2]"},{"internalType":"bytes32[][2]","name":"instPaths","type":"bytes32[][2]"},{"internalType":"bool[][2]","name":"instPathIsLefts","type":"bool[][2]"},{"internalType":"bytes32[2]","name":"instRoots","type":"bytes32[2]"},{"internalType":"bytes32[2]","name":"blkData","type":"bytes32[2]"},{"internalType":"uint256[][2]","name":"sigIdxs","type":"uint256[][2]"},{"internalType":"uint8[][2]","name":"sigVs","type":"uint8[][2]"},{"internalType":"bytes32[][2]","name":"sigRs","type":"bytes32[][2]"},{"internalType":"bytes32[][2]","name":"sigSs","type":"bytes32[][2]"}],"name":"withdraw","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"","type":"address"},{"internalType":"address","name":"","type":"address"}],"name":"withdrawRequests","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"bytes32","name":"","type":"bytes32"}],"name":"withdrawed","outputs":[{"internalType":"bool","name":"","type":"bool"}],"stateMutability":"view","type":"function"},{"stateMutability":"payable","type":"receive"}]`
	BridgeShardID = 1
	EthAddrStr    = "0x0000000000000000000000000000000000000000"
)

// Bridge, PDE & Portal statuses for RPCs
const (
	BridgeRequestNotFoundStatus   = 0
	BridgeRequestProcessingStatus = 1
	BridgeRequestAcceptedStatus   = 2
	BridgeRequestRejectedStatus   = 3

	PDENotFoundStatus = 0

	PDEContributionWaitingStatus          = 1
	PDEContributionAcceptedStatus         = 2
	PDEContributionRefundStatus           = 3
	PDEContributionMatchedNReturnedStatus = 4

	PDETradeAcceptedStatus = 1
	PDETradeRefundStatus   = 2

	PDEWithdrawalAcceptedStatus = 1
	PDEWithdrawalRejectedStatus = 2

	MinTxFeesOnTokenRequirement                             = 10000000000000 // 10000 prv, this requirement is applied from beacon height 87301 mainnet
	BeaconBlockHeighMilestoneForMinTxFeesOnTokenRequirement = 87301          // milestone of beacon height, when apply min fee on token requirement

	//portal
	PortalCustodianDepositAcceptedStatus = 1
	PortalCustodianDepositRefundStatus   = 2

	PortalReqPTokenAcceptedStatus = 1
	PortalReqPTokenRejectedStatus = 2

	PortalPortingTxRequestAcceptedStatus = 1
	PortalPortingTxRequestRejectedStatus = 3

	PortalPortingReqSuccessStatus    = 1
	PortalPortingReqWaitingStatus    = 2
	PortalPortingReqExpiredStatus    = 3
	PortalPortingReqLiquidatedStatus = 4

	PortalRedeemReqSuccessStatus               = 1
	PortalRedeemReqWaitingStatus               = 2
	PortalRedeemReqLiquidatedStatus            = 3
	PortalRedeemReqRejectedStatus              = 4
	PortalRedeemReqRejectedByLiquidationStatus = 5

	PortalCustodianWithdrawReqAcceptedStatus = 1
	PortalCustodianWithdrawReqRejectStatus   = 2

	PortalReqUnlockCollateralAcceptedStatus = 1
	PortalReqUnlockCollateralRejectedStatus = 2

	PortalLiquidateCustodianSuccessStatus = 1
	PortalLiquidateCustodianFailedStatus  = 2

	PortalLiquidationTPExchangeRatesSuccessStatus = 1
	PortalLiquidationTPExchangeRatesFailedStatus  = 2

	PortalReqWithdrawRewardAcceptedStatus = 1
	PortalReqWithdrawRewardRejectedStatus = 2

	PortalRedeemLiquidateExchangeRatesSuccessStatus  = 1
	PortalRedeemLiquidateExchangeRatesRejectedStatus = 2

	PortalLiquidationCustodianDepositSuccessStatus  = 1
	PortalLiquidationCustodianDepositRejectedStatus = 2

	PortalExpiredPortingReqSuccessStatus = 1
	PortalExpiredPortingReqFailedStatus  = 2

	PortalExchangeRatesAcceptedStatus = 1
	PortalExchangeRatesRejectedStatus = 2
)

// PDE statuses for chain
const (
	PDEContributionWaitingChainStatus          = "waiting"
	PDEContributionMatchedChainStatus          = "matched"
	PDEContributionRefundChainStatus           = "refund"
	PDEContributionMatchedNReturnedChainStatus = "matchedNReturned"

	PDETradeAcceptedChainStatus = "accepted"
	PDETradeRefundChainStatus   = "refund"

	PDEWithdrawalAcceptedChainStatus = "accepted"
	PDEWithdrawalRejectedChainStatus = "rejected"
)

// Portal status for chain
const (
	PortalCustodianDepositAcceptedChainStatus = "accepted"
	PortalCustodianDepositRefundChainStatus   = "refund"

	PortalReqPTokensAcceptedChainStatus = "accepted"
	PortalReqPTokensRejectedChainStatus = "rejected"

	PortalPortingRequestAcceptedChainStatus = "accepted"
	PortalPortingRequestRejectedChainStatus = "rejected"

	PortalExchangeRatesAcceptedChainStatus = "accepted"
	PortalExchangeRatesRejectedChainStatus = "rejected"

	PortalRedeemRequestAcceptedChainStatus              = "accepted"
	PortalRedeemRequestRejectedChainStatus              = "rejected"
	PortalRedeemRequestRejectedByLiquidationChainStatus = "rejectedByLiquidation"

	PortalCustodianWithdrawRequestAcceptedStatus = "accepted"
	PortalCustodianWithdrawRequestRejectedStatus = "rejected"

	PortalReqUnlockCollateralAcceptedChainStatus = "accepted"
	PortalReqUnlockCollateralRejectedChainStatus = "rejected"

	PortalLiquidateCustodianSuccessChainStatus = "success"
	PortalLiquidateCustodianFailedChainStatus  = "failed"

	PortalLiquidateTPExchangeRatesSuccessChainStatus = "success"
	PortalLiquidateTPExchangeRatesFailedChainStatus  = "rejected"

	PortalReqWithdrawRewardAcceptedChainStatus = "accepted"
	PortalReqWithdrawRewardRejectedChainStatus = "rejected"

	PortalRedeemLiquidateExchangeRatesSuccessChainStatus  = "success"
	PortalRedeemLiquidateExchangeRatesRejectedChainStatus = "rejected"

	PortalLiquidationCustodianDepositSuccessChainStatus  = "success"
	PortalLiquidationCustodianDepositRejectedChainStatus = "rejected"

	PortalExpiredWaitingPortingReqSuccessChainStatus = "success"
	PortalExpiredWaitingPortingReqFailedChainStatus  = "failed"
)

// Relaying header
const (
	RelayingHeaderRejectedChainStatus    = "rejected"
	RelayingHeaderConsideringChainStatus = "considering"
)

const PortalBTCIDStr = "b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696"
const PortalBNBIDStr = "b2655152784e8639fa19521a7035f331eea1f1e911b2f3200a507ebb4554387b"
const PRVIDStr = "0000000000000000000000000000000000000000000000000000000000000004"

var PortalSupportedIncTokenIDs = []string{
	"b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696", // pBTC
	"b2655152784e8639fa19521a7035f331eea1f1e911b2f3200a507ebb4554387b", // pBNB
}

// set MinAmountPortalPToken to avoid attacking with amount is less than smallest unit of cryptocurrency
// such as satoshi in BTC
var MinAmountPortalPToken = map[string]uint64{
	"b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696": 10,
	"b2655152784e8639fa19521a7035f331eea1f1e911b2f3200a507ebb4554387b": 10,
}

const (
	HexEmptyRoot = "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
)

var (
	EmptyRoot = HexToHash(HexEmptyRoot)
)
