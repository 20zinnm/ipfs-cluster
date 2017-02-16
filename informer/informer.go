package informer

import (
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/ipfscluster"
)

// Informer provides Metric information from a peer. The metrics produced by
// informers are then passed to a PinAllocator which will use them to
// determine where to pin content. The metric is agnostic to the rest of
// Cluster.
type Informer interface {
	ipfscluster.Component
	Name() string
	GetMetric() api.Metric
}