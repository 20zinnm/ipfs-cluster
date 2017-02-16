package allocator

import (
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/ipfscluster"
	peer "github.com/libp2p/go-libp2p-peer"
)

// PinAllocator decides where to pin certain content.
//
// In order to make such decision, it receives the pin arguments, the peers which are currently  allocated to the content and metrics available for all peers which could allocate the content.
type PinAllocator interface {
	ipfscluster.Component
	// Allocate returns the list of peers that should be assigned to
	// Pin content in oder of preference (from the most preferred to the
	// least). The "current" map contains valid metrics for peers
	// which are currently pinning the content. The candidates map
	// contains the metrics for all peers which are eligible for pinning
	// the content.
	Allocate(c *cid.Cid, current, candidates map[peer.ID]api.Metric) ([]peer.ID, error)
}
