package rpcservice

import (
	"fmt"

	"github.com/pkg/errors"
)

const (
	UnexpectedError = iota
	AlreadyStartedError
	JsonError

	RPCInvalidRequestError
	RPCMethodNotFoundError
	RPCInvalidParamsError
	RPCInvalidMethodPermissionError
	RPCInternalError
	RPCParseError

	InvalidTypeError
	AuthFailError
	InvalidSenderPrivateKeyError
	InvalidSenderViewingKeyError
	InvalidReceiverPaymentAddressError
	ListTokenNotFoundError
	CanNotSignError
	GetOutputCoinError
	CreateTxDataError
	SendTxDataError
	Base58ChedkDataOfTxInvalid
	JsonDataOfTxInvalid
	TxTypeInvalidError
	TxNotExistedInMemAndBLockError
	UnsubcribeError
	SubcribeError
	NetworkError
	TokenIsInvalidError
	GetClonedBeaconBestStateError
	GetBeaconBestBlockHashError
	GetBeaconBestBlockError
	GetClonedShardBestStateError
	GetShardBlockByHeightError
	GetShardBestBlockError
	GetShardBlockByHashError
	GetBeaconBlockByHashError
	GetBeaconBlockByHeightError
	GeTxFromPoolError
	NoSwapConfirmInst
	GetKeySetFromPrivateKeyError
	GetPDEStateError
	ListCommitteeRewardError
	GetRewardAmountError
	ListOutputCoinsByKeyError
	ListUnspentOutputCoinsByKeyError
	SendRawTransactionError
	BuildTokenParamError
	BuildPrivacyTokenParamError
	GetListPrivacyCustomTokenBalanceError
	// reject tx
	RejectInvalidTxFeeError
	RejectInvalidTxSizeError
	RejectInvalidTxTypeError
	RejectInvalidTxError
	RejectDoubleSpendTxError
	RejectDuplicateTxInPoolError
	RejectInvalidTxVersionError
	RejectSanityTxLocktime
	RejectReplacementTx
	TxPoolRejectTxError
	RejectInvalidFeeError
)

// Standard JSON-RPC 2.0 errors.
var ErrCodeMessage = map[int]struct {
	Code    int
	Message string
}{
	// general
	UnexpectedError:     {-1, "Unexpected error"},
	AlreadyStartedError: {-2, "RPC server is already started"},
	NetworkError:        {-3, "Network Error, failed to send request to RPC server"},
	JsonError:           {-4, "Json error"},

	// validate component -1xxx
	RPCInvalidRequestError:                {-1001, "Invalid request"},
	RPCMethodNotFoundError:                {-1002, "Method not found"},
	RPCInvalidParamsError:                 {-1003, "Invalid parameters"},
	RPCInternalError:                      {-1004, "Internal error"},
	RPCParseError:                         {-1005, "Parse error"},
	InvalidTypeError:                      {-1006, "Invalid type"},
	AuthFailError:                         {-1007, "Auth failure"},
	RPCInvalidMethodPermissionError:       {-1008, "Invalid method permission"},
	InvalidReceiverPaymentAddressError:    {-1009, "Invalid receiver paymentaddress"},
	ListTokenNotFoundError:                {-1010, "Can not find any token"},
	CanNotSignError:                       {-1011, "Can not sign with key"},
	InvalidSenderPrivateKeyError:          {-1012, "Invalid sender's key"},
	GetOutputCoinError:                    {-1013, "Can not get output coin"},
	TxTypeInvalidError:                    {-1014, "Invalid tx type"},
	InvalidSenderViewingKeyError:          {-1015, "Invalid viewing key"},
	RejectInvalidTxFeeError:               {-1016, "Reject invalid fee"},
	TxNotExistedInMemAndBLockError:        {-1017, "Tx is not existed in mem and block"},
	TokenIsInvalidError:                   {-1018, "Token is invalid"},
	GetKeySetFromPrivateKeyError:          {-1019, "Get KeySet From Private Key Error"},
	GetListPrivacyCustomTokenBalanceError: {-1020, "Get List Privacy Custom Token Balance Error"},
	// for block -2xxx
	GetShardBlockByHeightError:  {-2000, "Get shard block by height error"},
	GetShardBlockByHashError:    {-2001, "Get shard block by hash error"},
	GetShardBestBlockError:      {-2002, "Get shard best block error"},
	GetBeaconBlockByHashError:   {-2003, "Get beacon block by hash error"},
	GetBeaconBlockByHeightError: {-2004, "Get beacon block by height error"},
	GetBeaconBestBlockHashError: {-2004, "Get beacon best block hash error"},
	GetBeaconBestBlockError:     {-2005, "Get beacon best block error"},

	// best state -3xxx
	GetClonedBeaconBestStateError: {-3000, "Get Cloned Beacon Best State Error"},
	GetClonedShardBestStateError:  {-3001, "Get Cloned Shard Best State Error"},

	// tx -4xxx
	CreateTxDataError:                {-4001, "Can not create tx"},
	SendTxDataError:                  {-4002, "Can not send tx"},
	Base58ChedkDataOfTxInvalid:       {-4003, "Base58Check encode data of tx is invalid, can not decode"},
	JsonDataOfTxInvalid:              {-4004, "Json string data of tx is invalid, can not unmarshal"},
	ListOutputCoinsByKeyError:        {-4005, "List Output Coins By Key Error"},
	ListUnspentOutputCoinsByKeyError: {-4006, "List Unspent Output Coins By Key Error"},
	SendRawTransactionError:          {-4007, "Send Raw Transaction Error"},
	BuildTokenParamError:             {-4008, "Build Token Param Error"},
	BuildPrivacyTokenParamError:      {-4009, "Build Privacy Token Param Error"},
	// socket/subcribe -5xxx
	SubcribeError:   {-5000, "Failed to subcribe"},
	UnsubcribeError: {-5001, "Failed to unsubcribe"},

	// tx pool -6xxx
	GeTxFromPoolError:            {-6000, "Get tx from mempool error"},
	TxPoolRejectTxError:          {-6001, "Pool reject tx by unexpected error"},
	RejectInvalidTxSizeError:     {-6002, "Pool reject tx by invalid size"},
	RejectInvalidTxTypeError:     {-6003, "Pool reject tx by invalid type"},
	RejectInvalidTxError:         {-6004, "Pool reject invalid tx: signature, or proof or verify by itself fail"},
	RejectDoubleSpendTxError:     {-6005, "Pool reject double spend tx, double spend with blockchain or mempool"},
	RejectDuplicateTxInPoolError: {-6006, "Tx already exist in pool"},
	RejectInvalidTxVersionError:  {-6007, "Reject tx by invalid version"},
	RejectSanityTxLocktime:       {-6008, "Reject wrong tx by locktime"},
	RejectReplacementTx:          {-6009, "Reject error replacement or cancel transaction"},
	RejectInvalidFeeError:        {-6010, "Reject Invalid Fee Error"},

	// decentralized bridge
	NoSwapConfirmInst: {-7000, "No swap confirm instruction found in block"},

	// pde
	GetPDEStateError: {-8000, "Get pde state error"},
}

// RPCError represents an error that is used as a part of a JSON-RPC JsonResponse
// object.
type RPCError struct {
	Code       int    `json:"Code,omitempty"`
	Message    string `json:"Message,omitempty"`
	StackTrace string `json:"StackTrace"`

	err error `json:"Err"`
}

func GetErrorCode(err int) int {
	return ErrCodeMessage[err].Code
}

// Guarantee RPCError satisifies the builtin error interface.
var _, _ error = RPCError{}, (*RPCError)(nil)

// Error returns a string describing the RPC error.  This satisifies the
// builtin error interface.
func (e RPCError) Error() string {
	return fmt.Sprintf("%d: %+v %+v", e.Code, e.err, e.StackTrace)
}

func (e RPCError) GetErr() error {
	return e.err
}

// NewRPCError constructs and returns a new JSON-RPC error that is suitable
// for use in a JSON-RPC JsonResponse object.
func NewRPCError(key int, err error, param ...interface{}) *RPCError {
	e := &RPCError{
		Code: ErrCodeMessage[key].Code,
		err:  errors.Wrap(err, ErrCodeMessage[key].Message),
	}
	if len(param) > 0 {
		e.Message = fmt.Sprintf(ErrCodeMessage[key].Message, param)
	} else {
		e.Message = ErrCodeMessage[key].Message
	}
	return e
}

// internalRPCError is a convenience function to convert an internal error to
// an RPC error with the appropriate Code set.  It also logs the error to the
// RPC server subsystem since internal errors really should not occur.  The
// context parameter is only used in the log Message and may be empty if it's
// not needed.
func InternalRPCError(errStr, context string) *RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	Logger.log.Info(logStr)
	return NewRPCError(RPCInternalError, errors.New(errStr))
}
