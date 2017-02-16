// //Package numpinalloc implements an ipfscluster.Allocator based on the "numpin"
// //Informer. It is a simple example on how an allocator is implemented.
package allocator

import (
	"sort"
	"strconv"

	rpc "github.com/hsanjuan/go-libp2p-gorpc"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/informer"
	peer "github.com/libp2p/go-libp2p-peer"
)

// allocator implements Allocator.
type allocator struct{}

// NumPin returns an initialized allocator
func NumPin() *allocator {
	return &allocator{}
}

// SetClient does nothing in this allocator
func (alloc *allocator) SetClient(c *rpc.Client) {}

// Shutdown does nothing in this allocator
func (alloc *allocator) Shutdown() error { return nil }

// Allocate returns where to allocate a pin request based on "numpin"-Informer
// metrics. In this simple case, we do not pay attention to the metrics
// of the current, we just need to sort the candidates by number of pins.
func (alloc *allocator) Allocate(c *cid.Cid, current, candidates map[peer.ID]api.Metric) ([]peer.ID, error) {
	// sort our metrics
	numpins := newMetricsSorter(candidates)
	sort.Sort(numpins)
	return numpins.peers, nil
}

// metricsSorter attaches sort.Interface methods to our metrics and sorts
// a slice of peers in the way that interest us
type metricsSorter struct {
	peers []peer.ID
	m     map[peer.ID]int
}

func newMetricsSorter(m map[peer.ID]api.Metric) *metricsSorter {
	vMap := make(map[peer.ID]int)
	peers := make([]peer.ID, 0, len(m))
	for k, v := range m {
		if v.Name != informer.NumpinMetricName || v.Discard() {
			continue
		}
		val, err := strconv.Atoi(v.Value)
		if err != nil {
			continue
		}
		peers = append(peers, k)
		vMap[k] = val
	}

	sorter := &metricsSorter{
		m:     vMap,
		peers: peers,
	}
	return sorter
}

// Len returns the number of metrics
func (s metricsSorter) Len() int {
	return len(s.peers)
}

// Less reports if the element in position i is less than the element in j
func (s metricsSorter) Less(i, j int) bool {
	peeri := s.peers[i]
	peerj := s.peers[j]

	x := s.m[peeri]
	y := s.m[peerj]

	return x < y
}

// Swap swaps the elements in positions i and j
func (s metricsSorter) Swap(i, j int) {
	temp := s.peers[i]
	s.peers[i] = s.peers[j]
	s.peers[j] = temp
}
