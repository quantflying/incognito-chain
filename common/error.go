package common

const (
	UnexpectedErr = iota
	Base58CheckDecodeErr
)

var ErrCodeMessage = map[int]struct {
	code    int
	message string
}{
	UnexpectedErr:        {-10001, "Unexpected error"},
	Base58CheckDecodeErr: {-10002, "Unexpected error"},
}
