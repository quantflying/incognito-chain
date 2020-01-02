package shard

type StoreDatabaseState struct {
	block *ShardBlock
}

func (s *ShardView) StoreDatabase(block *ShardBlock) error {
	state := StoreDatabaseState{block: block}

	return nil
}
