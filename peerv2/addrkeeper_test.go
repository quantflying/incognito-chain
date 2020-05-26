package peerv2

import (
	"testing"

	"github.com/incognitochain/incognito-chain/peerv2/mocks"
	"github.com/incognitochain/incognito-chain/peerv2/rpcclient"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChooseHighwayFiltered(t *testing.T) {
	hwAddrs := []rpcclient.HighwayAddr{
		rpcclient.HighwayAddr{Libp2pAddr: testHighwayAddress},
		rpcclient.HighwayAddr{Libp2pAddr: ""},
	}
	keeper := NewAddrKeeper()
	keeper.addrs = hwAddrs

	pid := peer.ID("")
	_, err := keeper.chooseHighwayFromList(pid)
	assert.Nil(t, err)
}

func TestChooseHighwayFromSortedList(t *testing.T) {
	addr1 := "/ip4/0.0.0.0/tcp/7337/p2p/QmSPa4gxx6PRmoNRu6P2iFwEwmayaoLdR5By3i3MgM9gMv"
	addr2 := "/ip4/0.0.0.1/tcp/7337/p2p/QmSPa4gxx6PRmoNRu6P2iFwEwmayaoLdR5By3i3MgM9gMv"
	addr3 := "/ip4/0.0.1.0/tcp/7337/p2p/QmSPa4gxx6PRmoNRu6P2iFwEwmayaoLdR5By3i3MgM9gMv"
	hwAddrs1 := []rpcclient.HighwayAddr{
		rpcclient.HighwayAddr{Libp2pAddr: addr1},
		rpcclient.HighwayAddr{Libp2pAddr: addr2},
		rpcclient.HighwayAddr{Libp2pAddr: addr3},
	}
	keeper := NewAddrKeeper()
	keeper.addrs = hwAddrs1

	pid := peer.ID("")
	info1, err := keeper.chooseHighwayFromList(pid)
	assert.Nil(t, err)

	hwAddrs2 := []rpcclient.HighwayAddr{
		rpcclient.HighwayAddr{Libp2pAddr: addr3},
		rpcclient.HighwayAddr{Libp2pAddr: addr2},
		rpcclient.HighwayAddr{Libp2pAddr: addr1},
	}
	keeper.addrs = hwAddrs2

	info2, err := keeper.chooseHighwayFromList(pid)
	assert.Nil(t, err)
	assert.Equal(t, info1, info2)
}

func TestChoosePeerConsistent(t *testing.T) {
	addr1 := "/ip4/0.0.0.0/tcp/7337/p2p/QmSPa4gxx6PRmoNRu6P2iFwEwmayaoLdR5By3i3MgM9gMv"
	addr2 := "/ip4/0.0.0.0/tcp/7337/p2p/QmRWYJ1E6uXzBuY93iMkSDTSdF9XMzLhYcZKwQLLjKV2LW"
	addr3 := "/ip4/0.0.0.0/tcp/7337/p2p/QmQT92nmuhYbRHn6pbrHF2naWSerVaqmWFrEk8p5NfFWST"
	hwAddrs := []rpcclient.HighwayAddr{
		rpcclient.HighwayAddr{Libp2pAddr: addr1},
		rpcclient.HighwayAddr{Libp2pAddr: addr2},
		rpcclient.HighwayAddr{Libp2pAddr: addr3},
	}
	pid := peer.ID("")
	info, err := choosePeer(hwAddrs, pid)
	assert.Nil(t, err)
	assert.Equal(t, hwAddrs[1], info)
}

func TestGetHighwayAddrsRandomly(t *testing.T) {
	resultAddrs := map[string][]rpcclient.HighwayAddr{}
	discoverer := &mocks.HighwayDiscoverer{}
	rpcUsed := map[string]int{}
	discoverer.On("DiscoverHighway", mock.Anything, mock.Anything).Return(resultAddrs, nil).Run(
		func(args mock.Arguments) {
			rpcUsed[args.Get(0).(string)] = 1
		},
	)
	hwAddrs := []rpcclient.HighwayAddr{rpcclient.HighwayAddr{RPCUrl: "abc"}, rpcclient.HighwayAddr{RPCUrl: "xyz"}}
	keeper := NewAddrKeeper()
	keeper.addrs = hwAddrs

	for i := 0; i < 100; i++ {
		_, err := keeper.getHighwayAddrs(discoverer)
		assert.Nil(t, err)
	}
	assert.Len(t, rpcUsed, 2)
}

func TestGetAllHighways(t *testing.T) {
	hwAddrs := map[string][]rpcclient.HighwayAddr{
		"all": []rpcclient.HighwayAddr{rpcclient.HighwayAddr{Libp2pAddr: "abc"}, rpcclient.HighwayAddr{Libp2pAddr: "xyz"}},
		"1":   []rpcclient.HighwayAddr{rpcclient.HighwayAddr{Libp2pAddr: "123"}},
	}
	discoverer := &mocks.HighwayDiscoverer{}
	discoverer.On("DiscoverHighway", mock.Anything, mock.Anything).Return(hwAddrs, nil)
	hws, err := getAllHighways(discoverer, "")
	assert.Nil(t, err)
	assert.Equal(t, addresses(hwAddrs["all"]), hws)
}
