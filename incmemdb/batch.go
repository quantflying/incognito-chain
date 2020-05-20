package incmemdb

import "github.com/incognitochain/incognito-chain/incdb"

type Action struct {
	action int
	key    []byte
	value  []byte
}
type Batch struct {
	db      *db
	actions []Action
}

func (s *Batch) Put(key []byte, value []byte) error {
	s.actions = append(s.actions, Action{0, key, value})
	return nil
}

func (s *Batch) Delete(key []byte) error {
	s.actions = append(s.actions, Action{1, key, nil})
	return nil
}

func (s *Batch) ValueSize() int {
	return 0
}

func (s *Batch) Write() error {
	for _, act := range s.actions {
		if act.action == 0 {
			s.db.Put(act.key, act.value)
		}
		if act.action == 1 {
			s.db.Delete(act.key)
		}
	}
	return nil
}

func (s *Batch) Reset() {
	s.actions = []Action{}
}

func (s *Batch) Replay(w incdb.KeyValueWriter) error {
	for _, act := range s.actions {
		if act.action == 0 {
			w.Put(act.key, act.value)
		}
		if act.action == 1 {
			w.Delete(act.key)
		}
	}
	return nil
}
