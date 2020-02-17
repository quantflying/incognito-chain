package blockinterface

type ShardBlockInterface interface {
	GetBlockType() string
	GetBeaconHeight() uint64
}
