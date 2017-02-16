// Package numpin implements
package informer

import (
	"fmt"

	rpc "github.com/hsanjuan/go-libp2p-gorpc"

	"github.com/ipfs/ipfs-cluster/api"
)

// NumpinMetricTTL specifies how long the numpin metric is valid in seconds.
var NumpinMetricTTL = 10

// NumpinMetricName specifies the identifier of the numpin metric
var NumpinMetricName = "numpin"

// numpinInformer is a simple object to implement the ipfscluster.numpinInformer
// and Component interfaces
type numpinInformer struct {
	rpcClient *rpc.Client
}

// Numpin returns an ipfs-cluster informer which determines how many items this peer is pinning and returns it as an `api.Metric`
func Numpin(client *rpc.Client) *numpinInformer {
	return &numpinInformer{client}
}

// SetClient provides us with an rpc.Client which allows
// contacting other components in the cluster.
func (npi *numpinInformer) SetClient(c *rpc.Client) {
	npi.rpcClient = c
}

// Shutdown is called on cluster shutdown. We just invalidate
// any metrics from this point.
func (npi *numpinInformer) Shutdown() error {
	npi.rpcClient = nil
	return nil
}

// Name returns the name of this informer
func (npi *numpinInformer) Name() string {
	return NumpinMetricName
}

// GetMetric contacts the IPFSConnector component and
// requests the `pin ls` command. We return the number
// of pins in IPFS.
func (npi *numpinInformer) GetMetric() api.Metric {
	if npi.rpcClient == nil {
		return api.Metric{
			Valid: false,
		}
	}

	pinMap := make(map[string]api.IPFSPinStatus)

	// make use of the RPC API to obtain information
	// about the number of pins in IPFS. See RPCAPI docs.
	err := npi.rpcClient.Call("", // Local call
		"Cluster",            // Service name
		"IPFSPinLs",          // Method name
		"recursive",          // in arg
		&pinMap) // out arg

	valid := err == nil

	m := api.Metric{
		Name:  NumpinMetricName,
		Value: fmt.Sprintf("%d", len(pinMap)),
		Valid: valid,
	}

	m.SetTTL(NumpinMetricTTL)
	return m
}
