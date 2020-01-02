package v2

type BeaconBlockInterface interface {
	GetConfirmedCrossShardBlockToShard() map[byte]map[byte][]interface{}
}
