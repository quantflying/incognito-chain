package peerv2

import (
	"math/rand"
	"sort"
	"time"

	"github.com/incognitochain/incognito-chain/peerv2/rpcclient"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"stathat.com/c/consistent"
)

type addresses []rpcclient.HighwayAddr

// AddrKeeper stores all highway addresses for ConnManager to choose from.
// The address can be used to:
// 1. Make an RPC call to get a new list of highway
// 2. Choose a highway (consistent hashed) and connect to it
// For the 1st type, if it fails, AddrKeeper will ignore the requested address
// for some time so that the next few calls will be more likely to succeed.
// For the 2nd type, caller can manually ignore the chosen address.
type AddrKeeper struct {
	addrs          addresses
	ignoreRPCUntil map[rpcclient.HighwayAddr]time.Time
	ignoreHWUntil  map[rpcclient.HighwayAddr]time.Time
}

func NewAddrKeeper() *AddrKeeper {
	return &AddrKeeper{
		addrs:          addresses{},
		ignoreRPCUntil: map[rpcclient.HighwayAddr]time.Time{},
		ignoreHWUntil:  map[rpcclient.HighwayAddr]time.Time{},
	}
}

// ChooseHighway refreshes the list of highways by asking a random one and choose a (consistently) random highway to connect
func (keeper *AddrKeeper) ChooseHighway(discoverer HighwayDiscoverer, ourPID peer.ID) (rpcclient.HighwayAddr, error) {
	// Get a list of new highways
	newAddrs, err := keeper.getHighwayAddrs(discoverer)
	if err != nil {
		return rpcclient.HighwayAddr{}, err
	}

	// Update the local list of known highways
	keeper.addrs = mergeAddrs(keeper.addrs, newAddrs)
	Logger.Infof("Updated highway addresses: %+v", keeper.addrs)

	// Choose one and return
	chosenAddr, err := chooseHighwayFromList(keeper.addrs, ourPID)
	if err != nil {
		return rpcclient.HighwayAddr{}, err
	}
	return chosenAddr, nil
}

func (keeper *AddrKeeper) Add(addr rpcclient.HighwayAddr) {
	keeper.addrs = append(keeper.addrs, addr)
}

// mergeAddrs finds the union of N lists of highway addresses
// 2 addresses are considered the same when both RPCUrl and Libp2pAddr are the same
func mergeAddrs(allAddrs ...addresses) addresses {
	if len(allAddrs) == 0 {
		return nil
	}
	merged := allAddrs[0]
	for _, addrs := range allAddrs[1:] {
		for _, a := range addrs {
			found := false
			for _, b := range merged {
				if b.Libp2pAddr == a.Libp2pAddr && b.RPCUrl == a.RPCUrl {
					found = true
					break
				}
			}

			if !found {
				merged = append(merged, a)
			}
		}
	}
	return merged
}

// chooseHighwayFromList returns a random highway address from a list using consistent hashing; ourPID is the anchor of the hashing
func chooseHighwayFromList(hwAddrs addresses, ourPID peer.ID) (rpcclient.HighwayAddr, error) {
	if len(hwAddrs) == 0 {
		return rpcclient.HighwayAddr{}, errors.New("cannot choose highway from empty list")
	}

	// Filter out bootnode address (address with only rpcUrl)
	filterAddrs := addresses{}
	for _, addr := range hwAddrs {
		if len(addr.Libp2pAddr) != 0 {
			filterAddrs = append(filterAddrs, addr)
		}
	}

	// Sort first to make sure always choosing the same highway
	// if the list doesn't change
	// NOTE: this is redundant since hash key doesn't contain indexes
	// But we still keep it anyway to support other consistent hashing library
	sort.SliceStable(filterAddrs, func(i, j int) bool {
		return filterAddrs[i].Libp2pAddr < filterAddrs[j].Libp2pAddr
	})

	addr, err := choosePeer(filterAddrs, ourPID)
	if err != nil {
		return rpcclient.HighwayAddr{}, err
	}
	return addr, nil
}

// choosePeer picks a peer from a list using consistent hashing
func choosePeer(peers addresses, id peer.ID) (rpcclient.HighwayAddr, error) {
	cst := consistent.New()
	for _, p := range peers {
		cst.Add(p.Libp2pAddr)
	}

	closest, err := cst.Get(string(id))
	if err != nil {
		return rpcclient.HighwayAddr{}, errors.Errorf("could not get consistent-hashing peer %v %v", peers, id)
	}

	for _, p := range peers {
		if p.Libp2pAddr == closest {
			return p, nil
		}
	}
	return rpcclient.HighwayAddr{}, errors.Errorf("could not find closest peer %v %v %v", peers, id, closest)
}

// getHighwayAddrs picks a random highway, makes an RPC call to get an updated list of highways
// If fails, the picked address will be ignore for some time.
func (keeper *AddrKeeper) getHighwayAddrs(discoverer HighwayDiscoverer) (addresses, error) {
	if len(keeper.addrs) == 0 {
		return nil, errors.New("No peer to get list of highways")
	}

	// Pick random highway to make an RPC call
	addrs := getNonIgnoredAddrs(keeper.addrs, keeper.ignoreRPCUntil)
	if len(addrs) == 0 {
		// All ignored, pick random one
		addrs = keeper.addrs
	}
	addr := addrs[rand.Intn(len(addrs))]
	Logger.Infof("RPCing addr %v from list %v", addr, addrs)

	newAddrs, err := getAllHighways(discoverer, addr.RPCUrl)
	if err == nil {
		return newAddrs, nil
	}

	// Ignore for a while
	keeper.ignoreRPCUntil[addr] = time.Now().Add(IgnoreRPCDuration)
	Logger.Infof("Ignoring RPC of address %v until %s", addr, keeper.ignoreRPCUntil[addr].Format(time.RFC3339))
	return nil, err
}

func getNonIgnoredAddrs(addrs addresses, ignoreUntil map[rpcclient.HighwayAddr]time.Time) addresses {
	now := time.Now()
	valids := addresses{}
	for _, addr := range addrs {
		if deadline, ok := ignoreUntil[addr]; !ok || now.After(deadline) {
			valids = append(valids, addr)
		}
	}
	return valids
}

func getAllHighways(discoverer HighwayDiscoverer, rpcUrl string) (addresses, error) {
	mapHWPerShard, err := discoverer.DiscoverHighway(rpcUrl, []string{"all"})
	if err != nil {
		return nil, err
	}
	Logger.Infof("Got %v from bootnode", mapHWPerShard)
	return mapHWPerShard["all"], nil
}

func getAddressInfo(libp2pAddr string) (*peer.AddrInfo, error) {
	addr, err := multiaddr.NewMultiaddr(libp2pAddr)
	if err != nil {
		return nil, errors.WithMessagef(err, "invalid libp2p address: %s", libp2pAddr)
	}
	hwPeerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, errors.WithMessagef(err, "invalid multi address: %s", addr)
	}
	return hwPeerInfo, nil
}
