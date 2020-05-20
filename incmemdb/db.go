package incmemdb

import (
	"errors"
	"github.com/incognitochain/incognito-chain/incdb"
	cmap "github.com/orcaman/concurrent-map"
)

type db struct {
	inmemDB cmap.ConcurrentMap
}

func (s *db) Has(key []byte) (bool, error) {
	return s.inmemDB.Has(string(key)), nil
}

func (s *db) Get(key []byte) ([]byte, error) {
	v, ok := s.inmemDB.Get(string(key))
	if ok {
		return v.([]byte), nil
	}
	return nil, errors.New("Cannot find in database")
}

func (s *db) Put(key []byte, value []byte) error {
	s.inmemDB.Set(string(key), value)
	return nil
}

func (s *db) Delete(key []byte) error {
	s.inmemDB.Remove(string(key))
	return nil
}

func (s *db) NewBatch() incdb.Batch {
	return &Batch{db: s}
}

func (s *db) NewIterator() incdb.Iterator {
	panic("implement me")
}

func (s *db) NewIteratorWithStart(start []byte) incdb.Iterator {
	panic("implement me")
}

func (s *db) NewIteratorWithPrefix(prefix []byte) incdb.Iterator {
	panic("implement me")
}

func (s *db) Stat(property string) (string, error) {
	return "", nil
}

func (s *db) Compact(start []byte, limit []byte) error {
	return nil
}

func (s *db) Close() error {
	return nil
}
