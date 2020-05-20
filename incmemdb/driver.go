package incmemdb

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incdb"
	cmap "github.com/orcaman/concurrent-map"
)

// Open opens the db connection.
func OpenMultipleDB(typ string, dbPath string) (map[int]incdb.Database, error) {
	m := make(map[int]incdb.Database)
	for i := -1; i < common.MaxShardNumber; i++ {
		m[i] = &db{inmemDB: cmap.New()}
	}

	return m, nil
}
