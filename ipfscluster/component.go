package ipfscluster

import (
	rpc "github.com/hsanjuan/go-libp2p-gorpc"
	peer "github.com/libp2p/go-libp2p-peer"
)


// Component represents a piece of ipfscluster. Cluster components
// usually run their own goroutines (a http server for example). They
// communicate with the main Cluster component and other components
// (both local and remote), using an instance of rpc.Client.
type Component interface {
	SetClient(*rpc.Client)
	Shutdown() error
}

// Peered represents a component which needs to be aware of the peers
// in the Cluster and of any changes to the peer set.
type Peered interface {
	AddPeer(p peer.ID)
	RmPeer(p peer.ID)
	//SetPeers(peers []peer.ID)
}