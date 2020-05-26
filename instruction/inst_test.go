package instruction

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	s := NewRandomInst().SetNonce(100).SetBtcBlockHeight(100).SetBtcBlockTime(100).SetCheckpointTime(100)
	fmt.Println(s.ToString())
}
