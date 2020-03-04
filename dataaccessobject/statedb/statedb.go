package statedb

import (
	"encoding/json"
	"math/big"
	"strconv"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/trie"
)

// StateDBs within the incognito protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * State Object
type StateDB struct {
	db   DatabaseAccessWarper
	trie Trie
	//rawdb incdb.Database
	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects        map[common.Hash]StateObject
	stateObjectsPending map[common.Hash]struct{} // State objects finalized but not yet written to the trie
	stateObjectsDirty   map[common.Hash]struct{} // State objects modified in the current execution

	// DB error.
	// State objects are used by the consensus core which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Measurements gathered during execution for debugging purposes
	StateObjectReads   time.Duration
	StateObjectHashes  time.Duration
	StateObjectUpdates time.Duration
	StateObjectCommits time.Duration
}

//
//// New return a new statedb attach with a state root
//func New(root common.Hash, db DatabaseAccessWarper) (*StateDB, error) {
//	tr, err := db.OpenTrie(root)
//	if err != nil {
//		return nil, err
//	}
//	return &StateDB{
//		db:                  db,
//		trie:                tr,
//		stateObjects:        make(map[common.Hash]StateObject),
//		stateObjectsPending: make(map[common.Hash]struct{}),
//		stateObjectsDirty:   make(map[common.Hash]struct{}),
//	}, nil
//}
//
//// New return a new statedb attach with a state root
//func NewWithRawDB(root common.Hash, db DatabaseAccessWarper, rawdb incdb.Database) (*StateDB, error) {
//	tr, err := db.OpenTrie(root)
//	if err != nil {
//		return nil, err
//	}
//	return &StateDB{
//		db:                  db,
//		trie:                tr,
//		rawdb:               rawdb,
//		stateObjects:        make(map[common.Hash]StateObject),
//		stateObjectsPending: make(map[common.Hash]struct{}),
//		stateObjectsDirty:   make(map[common.Hash]struct{}),
//	}, nil
//}

// New return a new statedb attach with a state root
func NewWithPrefixTrie(root common.Hash, db DatabaseAccessWarper) (*StateDB, error) {
	tr, err := db.OpenPrefixTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:                  db,
		trie:                tr,
		stateObjects:        make(map[common.Hash]StateObject),
		stateObjectsPending: make(map[common.Hash]struct{}),
		stateObjectsDirty:   make(map[common.Hash]struct{}),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (stateDB *StateDB) setError(err error) {
	if stateDB.dbErr == nil {
		stateDB.dbErr = err
	}
}

// Error return statedb error
func (stateDB *StateDB) Error() error {
	return stateDB.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (stateDB *StateDB) Reset(root common.Hash) error {
	tr, err := stateDB.db.OpenPrefixTrie(root)
	if err != nil {
		return err
	}
	stateDB.trie = tr
	stateDB.stateObjects = make(map[common.Hash]StateObject)
	stateDB.stateObjectsPending = make(map[common.Hash]struct{})
	stateDB.stateObjectsDirty = make(map[common.Hash]struct{})
	return nil
}

func (stateDB *StateDB) ClearObjects() {
	stateDB.stateObjects = make(map[common.Hash]StateObject)
	stateDB.stateObjectsPending = make(map[common.Hash]struct{})
	stateDB.stateObjectsDirty = make(map[common.Hash]struct{})
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (stateDB *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	stateDB.markDeleteEmptyStateObject(deleteEmptyObjects)
	for addr := range stateDB.stateObjectsPending {
		obj := stateDB.stateObjects[addr]
		if obj.IsDeleted() {
			stateDB.deleteStateObject(obj)
		} else {
			stateDB.updateStateObject(obj)
		}
	}
	if len(stateDB.stateObjectsPending) > 0 {
		stateDB.stateObjectsPending = make(map[common.Hash]struct{})
	}
	// Track the amount of time wasted on hashing the account trie
	// if metrics.EnabledExpensive {
	// 	defer func(start time.Time) { stateDB.StateObjectHashes += time.Since(start) }(time.Now())
	// }
	return stateDB.trie.Hash()
}
func (stateDB *StateDB) markDeleteEmptyStateObject(deleteEmptyObjects bool) {
	for _, object := range stateDB.stateObjects {
		if object.IsEmpty() {
			object.MarkDelete()
		}
	}
}

// Commit writes the state to the underlying in-memory trie database.
func (stateDB *StateDB) Commit(deleteEmptyObjects bool) (common.Hash, error) {
	// Finalize any pending changes and merge everything into the tries
	stateDB.IntermediateRoot(deleteEmptyObjects)

	if len(stateDB.stateObjectsDirty) > 0 {
		stateDB.stateObjectsDirty = make(map[common.Hash]struct{})
	}
	// Write the account trie changes, measuing the amount of wasted time
	// if metrics.EnabledExpensive {
	// 	defer func(start time.Time) {
	// 		elapsed := time.Since(start)
	// 		stateDB.StateObjectCommits += elapsed
	// 		dataaccessobject.Logger.Log.Infof("StateDB commit and return root hash time %+v", elapsed)
	// 	}(time.Now())
	// }
	return stateDB.trie.Commit(func(leaf []byte, parent common.Hash) error {
		return nil
	})
}

// Database return current database access warper
func (stateDB *StateDB) Database() DatabaseAccessWarper {
	return stateDB.db
}

// Copy duplicate statedb and return new statedb instance
func (stateDB *StateDB) Copy() *StateDB {
	return &StateDB{
		db:                  stateDB.db,
		trie:                stateDB.db.CopyTrie(stateDB.trie),
		stateObjects:        make(map[common.Hash]StateObject),
		stateObjectsPending: make(map[common.Hash]struct{}),
		stateObjectsDirty:   make(map[common.Hash]struct{}),
	}
}

// Exist check existence of a state object in statedb
func (stateDB *StateDB) Exist(objectType int, stateObjectHash common.Hash) (bool, error) {
	value, err := stateDB.getStateObject(objectType, stateObjectHash)
	if err != nil {
		return false, err
	}
	return value != nil, nil
}

// Empty check a state object in statedb is empty or not
func (stateDB *StateDB) Empty(objectType int, stateObjectHash common.Hash) bool {
	stateObject, err := stateDB.getStateObject(objectType, stateObjectHash)
	return stateObject == nil || stateObject.IsEmpty() || err != nil
}

// ================================= STATE OBJECT =======================================
// getDeletedStateObject is similar to getStateObject, but instead of returning
// nil for a deleted state object, it returns the actual object with the deleted
// flag set. This is needed by the state journal to revert to the correct self-
// destructed object instead of wiping all knowledge about the state object.
func (stateDB *StateDB) getDeletedStateObject(objectType int, hash common.Hash) (StateObject, error) {
	// Prefer live objects if any is available
	if obj := stateDB.stateObjects[hash]; obj != nil {
		return obj, nil
	}
	// Track the amount of time wasted on loading the object from the database
	// if metrics.EnabledExpensive {
	// 	defer func(start time.Time) { stateDB.StateObjectReads += time.Since(start) }(time.Now())
	// }
	// Load the object from the database
	enc, err := stateDB.trie.TryGet(hash[:])
	if len(enc) == 0 {
		stateDB.setError(err)
		return nil, nil
	}
	newValue := make([]byte, len(enc))
	copy(newValue, enc)
	// Insert into the live set
	obj, err := newStateObjectWithValue(stateDB, objectType, hash, newValue)
	if err != nil {
		return nil, err
	}
	stateDB.setStateObject(obj)
	return obj, nil
}

// updateStateObject writes the given object to the trie.
func (stateDB *StateDB) updateStateObject(obj StateObject) {
	// Track the amount of time wasted on updating the account from the trie
	// if metrics.EnabledExpensive {
	// 	defer func(start time.Time) { stateDB.StateObjectUpdates += time.Since(start) }(time.Now())
	// }
	// Encode the account and update the account trie
	addr := obj.GetHash()
	data := obj.GetValueBytes()
	stateDB.setError(stateDB.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (stateDB *StateDB) deleteStateObject(obj StateObject) {
	// Track the amount of time wasted on deleting the account from the trie
	// if metrics.EnabledExpensive {
	// 	defer func(start time.Time) { stateDB.StateObjectUpdates += time.Since(start) }(time.Now())
	// }
	// Delete the account from the trie
	addr := obj.GetHash()
	stateDB.setError(stateDB.trie.TryDelete(addr[:]))
}

// createStateObject creates a new state object. If there is an existing account with
// the given hash, it is overwritten and returned as the second return value.
func (stateDB *StateDB) createStateObject(objectType int, hash common.Hash) (newobj, prev StateObject, err error) {
	prev, err = stateDB.getDeletedStateObject(objectType, hash) // Note, prev might have been deleted, we need that!
	if err != nil {
		return nil, nil, err
	}
	newobj = newStateObject(stateDB, objectType, hash)
	stateDB.stateObjectsPending[hash] = struct{}{}
	stateDB.setStateObject(newobj)
	return newobj, prev, err
}

func (stateDB *StateDB) createStateObjectWithValue(objectType int, hash common.Hash, value interface{}) (newobj, prev StateObject, err error) {
	newobj, err = newStateObjectWithValue(stateDB, objectType, hash, value)
	if err != nil {
		return nil, nil, err
	}
	stateDB.stateObjectsPending[hash] = struct{}{}
	stateDB.setStateObject(newobj)
	return newobj, prev, err
}

// SetStateObject add new stateobject into statedb
func (stateDB *StateDB) SetStateObject(objectType int, key common.Hash, value interface{}) error {
	obj, err := stateDB.getOrNewStateObjectWithValue(objectType, key, value)
	if err != nil {
		return err
	}
	err = obj.SetValue(value)
	if err != nil {
		return err
	}
	stateDB.stateObjectsPending[key] = struct{}{}
	return nil
}

// MarkDeleteStateObject add new stateobject into statedb
func (stateDB *StateDB) MarkDeleteStateObject(objectType int, key common.Hash) bool {
	stateObject, err := stateDB.getStateObject(objectType, key)
	if err == nil && stateObject != nil {
		stateObject.MarkDelete()
		stateDB.stateObjectsPending[key] = struct{}{}
		return true
	}
	return false
}

// Retrieve a state object or create a new state object if nil.
func (stateDB *StateDB) getOrNewStateObject(objectType int, hash common.Hash) (StateObject, error) {
	stateObject, err := stateDB.getStateObject(objectType, hash)
	if err != nil {
		return nil, err
	}
	if stateObject == nil {
		stateObject, _, err = stateDB.createStateObject(objectType, hash)
		if err != nil {
			return nil, err
		}
	}
	return stateObject, nil
}

func (stateDB *StateDB) getOrNewStateObjectWithValue(objectType int, hash common.Hash, value interface{}) (StateObject, error) {
	stateObject, err := stateDB.getStateObject(objectType, hash)
	if err != nil {
		return nil, err
	}
	if stateObject == nil {
		stateObject, _, err = stateDB.createStateObjectWithValue(objectType, hash, value)
		if err != nil {
			return nil, err
		}
	}
	return stateObject, nil
}

// add state object into statedb struct
func (stateDB *StateDB) setStateObject(object StateObject) {
	key := object.GetHash()
	stateDB.stateObjects[key] = object
}

// getStateObject retrieves a state object given by the address, returning nil if
// the object is not found or was deleted in this execution context. If you need
// to differentiate between non-existent/just-deleted, use getDeletedStateObject.
func (stateDB *StateDB) getStateObject(objectType int, addr common.Hash) (StateObject, error) {
	if obj, err := stateDB.getDeletedStateObject(objectType, addr); obj != nil && !obj.IsDeleted() {
		return obj, nil
	} else if err != nil {
		return nil, err
	}
	return nil, nil
}

// FOR TEST ONLY
// do not use this function for build feature
func (stateDB *StateDB) GetStateObjectMapForTestOnly() map[common.Hash]StateObject {
	return stateDB.stateObjects
}
func (stateDB *StateDB) GetStateObjectPendingMapForTestOnly() map[common.Hash]struct{} {
	return stateDB.stateObjectsPending
}

// =================================     Test Object     ========================================
func (stateDB *StateDB) GetTestObject(key common.Hash) ([]byte, error) {
	testObject, err := stateDB.getStateObject(TestObjectType, key)
	if err != nil {
		return []byte{}, err
	}
	if testObject != nil {
		return testObject.GetValueBytes(), nil
	}
	return []byte{}, nil
}
func (stateDB *StateDB) GetAllTestObjectList() ([]common.Hash, [][]byte) {
	temp := stateDB.trie.NodeIterator(nil)
	it := trie.NewIterator(temp)
	keys := []common.Hash{}
	values := [][]byte{}
	for it.Next() {
		key := stateDB.trie.GetKey(it.Key)
		newKey := make([]byte, len(key))
		copy(newKey, key)
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		keys = append(keys, common.BytesToHash(key))
		values = append(values, value)
	}
	return keys, values
}
func (stateDB *StateDB) GetAllTestObjectMap() map[common.Hash][]byte {
	temp := stateDB.trie.NodeIterator(nil)
	it := trie.NewIterator(temp)
	m := make(map[common.Hash][]byte)
	for it.Next() {
		key := stateDB.trie.GetKey(it.Key)
		newKey := make([]byte, len(key))
		copy(newKey, key)
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		m[common.BytesToHash(key)] = newValue
	}
	return m
}
func (stateDB *StateDB) GetByPrefixTestObjectList(prefix []byte) ([]common.Hash, [][]byte) {
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	keys := []common.Hash{}
	values := [][]byte{}
	for it.Next() {
		key := stateDB.trie.GetKey(it.Key)
		newKey := make([]byte, len(key))
		copy(newKey, key)
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		keys = append(keys, common.BytesToHash(key))
		values = append(values, value)
	}
	return keys, values
}

// ================================= Committee OBJECT =======================================
func (stateDB *StateDB) GetCommitteeState(key common.Hash) (*CommitteeState, bool, error) {
	committeeStateObject, err := stateDB.getStateObject(CommitteeObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if committeeStateObject != nil {
		return committeeStateObject.GetValue().(*CommitteeState), true, nil
	}
	return NewCommitteeState(), false, nil
}
func (stateDB *StateDB) GetAllValidatorCommitteePublicKey(role int, ids []int) map[int][]*CommitteeState {
	if role != CurrentValidator && role != SubstituteValidator {
		panic("wrong expected role " + strconv.Itoa(role))
	}
	m := make(map[int][]*CommitteeState)
	for _, id := range ids {
		prefix := GetCommitteePrefixWithRole(role, id)
		temp := stateDB.trie.NodeIterator(prefix)
		it := trie.NewIterator(temp)
		for it.Next() {
			value := it.Value
			newValue := make([]byte, len(value))
			copy(newValue, value)
			committeeState := NewCommitteeState()
			err := json.Unmarshal(newValue, committeeState)
			if err != nil {
				panic("wrong value type")
			}
			m[committeeState.shardID] = append(m[committeeState.shardID], committeeState)
		}
	}
	return m
}

func (stateDB *StateDB) GetAllCandidateCommitteePublicKey(role int) []*CommitteeState {
	if role != CurrentEpochShardCandidate && role != NextEpochShardCandidate {
		panic("wrong expected role " + strconv.Itoa(role))
	}
	list := []*CommitteeState{}
	prefix := GetCommitteePrefixWithRole(role, CandidateShardID)
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		committeeState := NewCommitteeState()
		err := committeeState.UnmarshalJSON(newValue)
		if err != nil {
			panic("wrong value type")
		}
		list = append(list, committeeState)
	}
	return list
}

func (stateDB *StateDB) GetByShardIDCurrentValidatorState(shardID int) []*CommitteeState {
	committees := []*CommitteeState{}
	prefix := GetCommitteePrefixWithRole(CurrentValidator, shardID)
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		committeeState := NewCommitteeState()
		err := json.Unmarshal(newValue, committeeState)
		if err != nil {
			panic("wrong value type")
		}
		if committeeState.ShardID() != shardID {
			panic("wrong expected shard id")
		}
		committees = append(committees, committeeState)
	}
	return committees
}

func (stateDB *StateDB) GetByShardIDSubstituteValidatorState(shardID int) []*CommitteeState {
	committees := []*CommitteeState{}
	prefix := GetCommitteePrefixWithRole(SubstituteValidator, shardID)
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		committeeState := NewCommitteeState()
		err := json.Unmarshal(newValue, committeeState)
		if err != nil {
			panic("wrong value type")
		}
		if committeeState.ShardID() != shardID {
			panic("wrong expected shard id")
		}
		committees = append(committees, committeeState)
	}
	return committees
}

// GetAllCommitteeState return all data related to all committee roles
// return params #1: current validator
// return params #2: substitute validator
// return params #3: next epoch candidate
// return params #4: current epoch candidate
// return params #5: reward receiver map
// return params #6: auto staking map
func (stateDB *StateDB) GetAllCommitteeState(ids []int) (map[int][]*CommitteeState, map[int][]*CommitteeState, []*CommitteeState, []*CommitteeState, []*CommitteeState, []*CommitteeState, map[string]string, map[string]bool) {
	currentValidator := make(map[int][]*CommitteeState)
	substituteValidator := make(map[int][]*CommitteeState)
	nextEpochShardCandidate := []*CommitteeState{}
	currentEpochShardCandidate := []*CommitteeState{}
	nextEpochBeaconCandidate := []*CommitteeState{}
	currentEpochBeaconCandidate := []*CommitteeState{}
	rewardReceivers := make(map[string]string)
	autoStaking := make(map[string]bool)
	for _, shardID := range ids {
		// Current Validator
		prefixCurrentValidator := GetCommitteePrefixWithRole(CurrentValidator, shardID)
		resCurrentValidator := stateDB.iterateWithCommitteeState(prefixCurrentValidator)
		tempCurrentValidator := []*CommitteeState{}
		for _, v := range resCurrentValidator {
			tempCurrentValidator = append(tempCurrentValidator, v)
			tempStr := v.CommitteePublicKey()
			tempCurrentValidatorString, err := tempStr.ToBase58()
			if err != nil {
				panic(err)
			}
			rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
			autoStaking[tempCurrentValidatorString] = v.autoStaking
		}
		currentValidator[shardID] = tempCurrentValidator
		// Substitute Validator
		prefixSubstituteValidator := GetCommitteePrefixWithRole(SubstituteValidator, shardID)
		resSubstituteValidator := stateDB.iterateWithCommitteeState(prefixSubstituteValidator)
		tempSubstituteValidator := []*CommitteeState{}
		for _, v := range resSubstituteValidator {
			tempSubstituteValidator = append(tempSubstituteValidator, v)
			tempStr := v.CommitteePublicKey()
			tempSubstituteValidatorString, err := tempStr.ToBase58()
			if err != nil {
				panic(err)
			}
			rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
			autoStaking[tempSubstituteValidatorString] = v.autoStaking
		}
		substituteValidator[shardID] = tempSubstituteValidator
	}
	// next epoch candidate
	prefixNextEpochCandidate := GetCommitteePrefixWithRole(NextEpochShardCandidate, -2)
	resNextEpochCandidate := stateDB.iterateWithCommitteeState(prefixNextEpochCandidate)
	for _, v := range resNextEpochCandidate {
		nextEpochShardCandidate = append(nextEpochShardCandidate, v)
		tempStr := v.CommitteePublicKey()
		tempNextEpochCandidateString, err := tempStr.ToBase58()
		if err != nil {
			panic(err)
		}
		rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
		autoStaking[tempNextEpochCandidateString] = v.autoStaking
	}
	// current epoch candidate
	prefixCurrentEpochCandidate := GetCommitteePrefixWithRole(CurrentEpochShardCandidate, -2)
	resCurrentEpochCandidate := stateDB.iterateWithCommitteeState(prefixCurrentEpochCandidate)
	for _, v := range resCurrentEpochCandidate {
		currentEpochShardCandidate = append(currentEpochShardCandidate, v)
		tempStr := v.CommitteePublicKey()
		tempCurrentEpochCandidateString, err := tempStr.ToBase58()
		if err != nil {
			panic(err)
		}
		rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
		autoStaking[tempCurrentEpochCandidateString] = v.autoStaking
	}

	// next epoch candidate
	prefixNextEpochBeaconCandidate := GetCommitteePrefixWithRole(NextEpochBeaconCandidate, -2)
	resNextEpochBeaconCandidate := stateDB.iterateWithCommitteeState(prefixNextEpochBeaconCandidate)
	for _, v := range resNextEpochBeaconCandidate {
		nextEpochBeaconCandidate = append(nextEpochBeaconCandidate, v)
		tempStr := v.CommitteePublicKey()
		tempNextEpochCandidateString, err := tempStr.ToBase58()
		if err != nil {
			panic(err)
		}
		rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
		autoStaking[tempNextEpochCandidateString] = v.autoStaking
	}
	// current epoch candidate
	prefixCurrentEpochBeaconCandidate := GetCommitteePrefixWithRole(CurrentEpochBeaconCandidate, -2)
	resCurrentEpochBeaconCandidate := stateDB.iterateWithCommitteeState(prefixCurrentEpochBeaconCandidate)
	for _, v := range resCurrentEpochBeaconCandidate {
		currentEpochBeaconCandidate = append(currentEpochBeaconCandidate, v)
		tempStr := v.CommitteePublicKey()
		tempCurrentEpochCandidateString, err := tempStr.ToBase58()
		if err != nil {
			panic(err)
		}
		rewardReceivers[tempStr.GetIncKeyBase58()] = v.rewardReceiver
		autoStaking[tempCurrentEpochCandidateString] = v.autoStaking
	}
	return currentValidator, substituteValidator, nextEpochShardCandidate, currentEpochShardCandidate, nextEpochBeaconCandidate, currentEpochBeaconCandidate, rewardReceivers, autoStaking
}

func (stateDB *StateDB) iterateWithCommitteeState(prefix []byte) []*CommitteeState {
	m := []*CommitteeState{}
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		committeeState := NewCommitteeState()
		err := json.Unmarshal(newValue, committeeState)
		if err != nil {
			panic("wrong value type")
		}
		m = append(m, committeeState)
	}
	return m
}

// ================================= Committee Reward OBJECT =======================================
func (stateDB *StateDB) GetCommitteeRewardState(key common.Hash) (*CommitteeRewardState, bool, error) {
	committeeRewardObject, err := stateDB.getStateObject(CommitteeRewardObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if committeeRewardObject != nil {
		return committeeRewardObject.GetValue().(*CommitteeRewardState), true, nil
	}
	return NewCommitteeRewardState(), false, nil
}

func (stateDB *StateDB) GetCommitteeRewardAmount(key common.Hash) (map[common.Hash]uint64, bool, error) {
	m := make(map[common.Hash]uint64)
	committeeRewardObject, err := stateDB.getStateObject(CommitteeRewardObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if committeeRewardObject != nil {
		temp := committeeRewardObject.GetValue().(*CommitteeRewardState)
		m = temp.reward
		return m, true, nil
	}
	return m, false, nil
}

func (stateDB *StateDB) GetAllCommitteeReward() map[string]map[common.Hash]uint64 {
	m := make(map[string]map[common.Hash]uint64)
	prefix := GetCommitteeRewardPrefix()
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		committeeRewardState := NewCommitteeRewardState()
		err := json.Unmarshal(newValue, committeeRewardState)
		if err != nil {
			panic("wrong value type")
		}
		m[committeeRewardState.incognitoPublicKey] = committeeRewardState.reward
	}
	return m
}

// ================================= Reward Request OBJECT =======================================
func (stateDB *StateDB) GetRewardRequestState(key common.Hash) (*RewardRequestState, bool, error) {
	rewardRequestState, err := stateDB.getStateObject(RewardRequestObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if rewardRequestState != nil {
		return rewardRequestState.GetValue().(*RewardRequestState), true, nil
	}
	return NewRewardRequestState(), false, nil
}

func (stateDB *StateDB) GetRewardRequestAmount(key common.Hash) (uint64, bool, error) {
	amount := uint64(0)
	rewardRequestObject, err := stateDB.getStateObject(RewardRequestObjectType, key)
	if err != nil {
		return amount, false, err
	}
	if rewardRequestObject != nil {
		temp := rewardRequestObject.GetValue().(*RewardRequestState)
		amount = temp.amount
		return amount, true, nil
	}
	return amount, false, nil
}

func (stateDB *StateDB) GetAllRewardRequestState(epoch uint64) ([]common.Hash, []*RewardRequestState) {
	m := []*RewardRequestState{}
	keys := []common.Hash{}
	prefix := GetRewardRequestPrefix(epoch)
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		key := it.Key
		newKey := make([]byte, len(key))
		copy(newKey, key)
		keys = append(keys, common.BytesToHash(newKey))
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		rewardRequestState := NewRewardRequestState()
		err := json.Unmarshal(newValue, rewardRequestState)
		if err != nil {
			panic("wrong value type")
		}
		m = append(m, rewardRequestState)
	}
	return keys, m
}

// ================================= Black List Producer OBJECT =======================================
func (stateDB *StateDB) GetBlackListProducerState(key common.Hash) (*BlackListProducerState, bool, error) {
	blackListProducerState, err := stateDB.getStateObject(BlackListProducerObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if blackListProducerState != nil {
		return blackListProducerState.GetValue().(*BlackListProducerState), true, nil
	}
	return NewBlackListProducerState(), false, nil
}

func (stateDB *StateDB) GetBlackListProducerPunishedEpoch(key common.Hash) (uint8, bool, error) {
	duration := uint8(0)
	blackListProducerObject, err := stateDB.getStateObject(BlackListProducerObjectType, key)
	if err != nil {
		return duration, false, err
	}
	if blackListProducerObject != nil {
		temp := blackListProducerObject.GetValue().(*BlackListProducerState)
		duration = temp.punishedEpoches
		return duration, true, nil
	}
	return duration, false, nil
}

func (stateDB *StateDB) GetAllBlackListProducerState() []*BlackListProducerState {
	blackListProducerStates := []*BlackListProducerState{}
	prefix := GetBlackListProducerPrefix()
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		blackListProducerState := NewBlackListProducerState()
		err := json.Unmarshal(newValue, blackListProducerState)
		if err != nil {
			panic("wrong value type")
		}
		blackListProducerStates = append(blackListProducerStates, blackListProducerState)
	}
	return blackListProducerStates
}

func (stateDB *StateDB) GetAllProducerBlackList() map[string]uint8 {
	m := make(map[string]uint8)
	prefix := GetBlackListProducerPrefix()
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		blackListProducerState := NewBlackListProducerState()
		err := json.Unmarshal(newValue, blackListProducerState)
		if err != nil {
			panic("wrong value type")
		}
		m[blackListProducerState.producerCommitteePublicKey] = blackListProducerState.punishedEpoches
	}
	return m
}

func (stateDB *StateDB) GetAllProducerBlackListState() map[common.Hash]uint8 {
	m := make(map[common.Hash]uint8)
	prefix := GetBlackListProducerPrefix()
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		key := it.Key
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		blackListProducerState := NewBlackListProducerState()
		err := json.Unmarshal(newValue, blackListProducerState)
		if err != nil {
			panic("wrong value type")
		}
		m[common.BytesToHash(key)] = blackListProducerState.punishedEpoches
	}
	return m
}

// ================================= Serial Number OBJECT =======================================
func (stateDB *StateDB) GetSerialNumberState(key common.Hash) (*SerialNumberState, bool, error) {
	serialNumberState, err := stateDB.getStateObject(SerialNumberObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if serialNumberState != nil {
		return serialNumberState.GetValue().(*SerialNumberState), true, nil
	}
	return NewSerialNumberState(), false, nil
}

func (stateDB *StateDB) GetAllSerialNumberByPrefix(tokenID common.Hash, shardID byte) [][]byte {
	serialNumberList := [][]byte{}
	prefix := GetSerialNumberPrefix(tokenID, shardID)
	temp := stateDB.trie.NodeIterator(prefix)
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		serialNumberState := NewSerialNumberState()
		err := json.Unmarshal(newValue, serialNumberState)
		if err != nil {
			panic("wrong value type")
		}
		serialNumberList = append(serialNumberList, serialNumberState.SerialNumber())
	}
	return serialNumberList
}

// ================================= Commitment OBJECT =======================================
func (stateDB *StateDB) GetCommitmentState(key common.Hash) (*CommitmentState, bool, error) {
	commitmentState, err := stateDB.getStateObject(CommitmentObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if commitmentState != nil {
		return commitmentState.GetValue().(*CommitmentState), true, nil
	}
	return NewCommitmentState(), false, nil
}
func (stateDB *StateDB) GetCommitmentIndexState(key common.Hash) (*CommitmentState, bool, error) {
	commitmentIndexState, err := stateDB.getStateObject(CommitmentIndexObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if commitmentIndexState != nil {
		tempKey, ok := commitmentIndexState.GetValue().(common.Hash)
		if !ok {
			panic("wrong expected type")
		}
		commitmentState, err := stateDB.getDeletedStateObject(CommitmentObjectType, tempKey)
		if err != nil || commitmentState == nil {
			return NewCommitmentState(), false, nil
		}
		return commitmentState.GetValue().(*CommitmentState), true, nil
	}
	return NewCommitmentState(), false, nil
}

func (stateDB *StateDB) GetCommitmentLengthState(key common.Hash) (*big.Int, bool, error) {
	commitmentLengthState, err := stateDB.getStateObject(CommitmentLengthObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if commitmentLengthState != nil {
		return commitmentLengthState.GetValue().(*big.Int), true, nil
	}
	return new(big.Int), false, nil
}

func (stateDB *StateDB) GetAllCommitmentStateByPrefix(tokenID common.Hash, shardID byte) map[string]uint64 {
	temp := stateDB.trie.NodeIterator(GetCommitmentPrefix(tokenID, shardID))
	it := trie.NewIterator(temp)
	m := make(map[string]uint64)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		newCommitmentState := NewCommitmentState()
		err := json.Unmarshal(newValue, newCommitmentState)
		if err != nil {
			panic("wrong expect type")
		}
		commitmentString := base58.Base58Check{}.Encode(newCommitmentState.Commitment(), common.Base58Version)
		m[commitmentString] = newCommitmentState.Index().Uint64()
	}
	return m
}

// ================================= Output Coin OBJECT =======================================
func (stateDB *StateDB) GetOutputCoinState(key common.Hash) (*OutputCoinState, bool, error) {
	outputCoinState, err := stateDB.getStateObject(OutputCoinObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if outputCoinState != nil {
		return outputCoinState.GetValue().(*OutputCoinState), true, nil
	}
	return NewOutputCoinState(), false, nil
}

// ================================= SNDerivator OBJECT =======================================
func (stateDB *StateDB) GetSNDerivatorState(key common.Hash) (*SNDerivatorState, bool, error) {
	sndState, err := stateDB.getStateObject(SNDerivatorObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if sndState != nil {
		return sndState.GetValue().(*SNDerivatorState), true, nil
	}
	return NewSNDerivatorState(), false, nil
}

func (stateDB *StateDB) GetAllSNDerivatorStateByPrefix(tokenID common.Hash) [][]byte {
	temp := stateDB.trie.NodeIterator(GetSNDerivatorPrefix(tokenID))
	it := trie.NewIterator(temp)
	list := [][]byte{}
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		newSNDerivatorState := NewSNDerivatorState()
		err := json.Unmarshal(newValue, newSNDerivatorState)
		if err != nil {
			panic("wrong expect type")
		}
		list = append(list, newSNDerivatorState.Snd())
	}
	return list
}

// ================================= Token OBJECT =======================================
func (stateDB *StateDB) GetTokenState(key common.Hash) (*TokenState, bool, error) {
	tokenState, err := stateDB.getStateObject(TokenObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if tokenState != nil {
		return tokenState.GetValue().(*TokenState), true, nil
	}
	return NewTokenState(), false, nil
}

func (stateDB *StateDB) GetTokenTxs(tokenID common.Hash) ([]common.Hash, bool, error) {
	t, has, err := stateDB.GetTokenState(tokenID)
	if err != nil {
		return []common.Hash{}, false, err
	}
	if !has {
		return []common.Hash{}, has, nil
	}
	return t.txs, true, nil
}

func (stateDB *StateDB) GetAllToken() map[common.Hash]*TokenState {
	temp := stateDB.trie.NodeIterator(GetTokenPrefix())
	it := trie.NewIterator(temp)
	tokenIDs := make(map[common.Hash]*TokenState)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		tokenState := NewTokenState()
		err := json.Unmarshal(newValue, tokenState)
		if err != nil {
			panic("wrong expect type")
		}
		tokenIDs[tokenState.TokenID()] = tokenState
	}
	return tokenIDs
}

// ================================= PDE OBJECT =======================================
func (stateDB *StateDB) GetAllWaitingPDEContributionState(beaconHeight uint64) []*WaitingPDEContributionState {
	waitingPDEContributionStates := []*WaitingPDEContributionState{}
	temp := stateDB.trie.NodeIterator(GetWaitingPDEContributionPrefix(beaconHeight))
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		wc := NewWaitingPDEContributionState()
		err := json.Unmarshal(newValue, wc)
		if err != nil {
			panic("wrong expect type")
		}
		waitingPDEContributionStates = append(waitingPDEContributionStates, wc)
	}
	return waitingPDEContributionStates
}

func (stateDB *StateDB) GetAllPDEPoolPairState(beaconHeight uint64) []*PDEPoolPairState {
	pdePoolPairStates := []*PDEPoolPairState{}
	temp := stateDB.trie.NodeIterator(GetPDEPoolPairPrefix(beaconHeight))
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		pp := NewPDEPoolPairState()
		err := json.Unmarshal(newValue, pp)
		if err != nil {
			panic("wrong expect type")
		}
		pdePoolPairStates = append(pdePoolPairStates, pp)
	}
	return pdePoolPairStates
}

func (stateDB *StateDB) GetPDEPoolPairState(key common.Hash) (*PDEPoolPairState, bool, error) {
	ppState, err := stateDB.getStateObject(PDEPoolPairObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if ppState != nil {
		return ppState.GetValue().(*PDEPoolPairState), true, nil
	}
	return NewPDEPoolPairState(), false, nil
}

func (stateDB *StateDB) GetAllPDEShareState(beaconHeight uint64) []*PDEShareState {
	pdeShareStates := []*PDEShareState{}
	temp := stateDB.trie.NodeIterator(GetPDESharePrefix(beaconHeight))
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		pp := NewPDEShareState()
		err := json.Unmarshal(newValue, pp)
		if err != nil {
			panic("wrong expect type")
		}
		pdeShareStates = append(pdeShareStates, pp)
	}
	return pdeShareStates
}

func (stateDB *StateDB) GetAllPDEStatus() []*PDEStatusState {
	pdeStatusStates := []*PDEStatusState{}
	temp := stateDB.trie.NodeIterator(GetPDEStatusPrefix())
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		s := NewPDEStatusState()
		err := json.Unmarshal(newValue, s)
		if err != nil {
			panic("wrong expect type")
		}
		pdeStatusStates = append(pdeStatusStates, s)
	}
	return pdeStatusStates
}

func (stateDB *StateDB) GetPDEStatusByKey(key common.Hash) (*PDEStatusState, bool, error) {
	pdeStatusState, err := stateDB.getStateObject(PDEStatusObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if pdeStatusState != nil {
		return pdeStatusState.GetValue().(*PDEStatusState), true, nil
	}
	return NewPDEStatusState(), false, nil
}

// ================================= Bridge OBJECT =======================================
func (stateDB *StateDB) GetBridgeEthTxState(key common.Hash) (*BridgeEthTxState, bool, error) {
	ethTxState, err := stateDB.getStateObject(BridgeEthTxObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if ethTxState != nil {
		return ethTxState.GetValue().(*BridgeEthTxState), true, nil
	}
	return NewBridgeEthTxState(), false, nil
}

func (stateDB *StateDB) GetBridgeTokenInfoState(key common.Hash) (*BridgeTokenInfoState, bool, error) {
	tokenInfoState, err := stateDB.getStateObject(BridgeTokenInfoObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if tokenInfoState != nil {
		return tokenInfoState.GetValue().(*BridgeTokenInfoState), true, nil
	}
	return NewBridgeTokenInfoState(), false, nil
}

func (stateDB *StateDB) GetAllBridgeTokenInfoState(isCentralized bool) []*BridgeTokenInfoState {
	bridgeTokenInfoStates := []*BridgeTokenInfoState{}
	temp := stateDB.trie.NodeIterator(GetBridgeTokenInfoPrefix(isCentralized))
	it := trie.NewIterator(temp)
	for it.Next() {
		value := it.Value
		newValue := make([]byte, len(value))
		copy(newValue, value)
		s := NewBridgeTokenInfoState()
		err := json.Unmarshal(newValue, s)
		if err != nil {
			panic("wrong expect type")
		}
		bridgeTokenInfoStates = append(bridgeTokenInfoStates, s)
	}
	return bridgeTokenInfoStates
}

func (stateDB *StateDB) GetBridgeStatusState(key common.Hash) (*BridgeStatusState, bool, error) {
	statusState, err := stateDB.getStateObject(BridgeStatusObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if statusState != nil {
		return statusState.GetValue().(*BridgeStatusState), true, nil
	}
	return NewBridgeStatusState(), false, nil
}

// ================================= Burn OBJECT =======================================
func (stateDB *StateDB) GetBurningConfirmState(key common.Hash) (*BurningConfirmState, bool, error) {
	burningConfirmState, err := stateDB.getStateObject(BridgeStatusObjectType, key)
	if err != nil {
		return nil, false, err
	}
	if burningConfirmState != nil {
		return burningConfirmState.GetValue().(*BurningConfirmState), true, nil
	}
	return NewBurningConfirmState(), false, nil
}
