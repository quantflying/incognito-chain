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

type AddrKeeper struct {
	addrs       addresses
	ignoreUntil map[rpcclient.HighwayAddr]time.Time
}

// ChooseHighway refreshes the list of highways by asking a random one and choose a (consistently) random highway to connect
func (keeper *AddrKeeper) ChooseHighway(discoverer HighwayDiscoverer, ourPID peer.ID) (*peer.AddrInfo, error) {
	// Get a list of new highways
	newAddrs, err := getHighwayAddrs(discoverer, keeper.addrs)
	if err != nil {
		return nil, err
	}

	// Update the local list of known highways
	keeper.addrs = mergeAddrs(keeper.addrs, newAddrs)
	Logger.Infof("Updated highway addresses: %+v", keeper.addrs)

	// Choose one and return
	chosePID, err := chooseHighwayFromList(keeper.addrs, ourPID)
	if err != nil {
		return nil, err
	}
	return chosePID, nil
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
func chooseHighwayFromList(hwAddrs addresses, ourPID peer.ID) (*peer.AddrInfo, error) {
	if len(hwAddrs) == 0 {
		return nil, errors.New("cannot choose highway from empty list")
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
		return nil, err
	}
	return getAddressInfo(addr.Libp2pAddr)
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

func getHighwayAddrs(discoverer HighwayDiscoverer, hwAddrs addresses) (addresses, error) {
	// Pick random peer to get new list of highways
	if len(hwAddrs) == 0 {
		return nil, errors.New("No peer to get list of highways")
	}
	addr := hwAddrs[rand.Intn(len(hwAddrs))]
	newAddrs, err := getAllHighways(discoverer, addr.RPCUrl)
	if err != nil {
		return nil, err
	}
	return newAddrs, nil
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
