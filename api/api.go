package api

import "github.com/ipfs/ipfs-cluster/ipfscluster"

// API is a component which offers an API for Cluster. This is a base component.
type API interface {
	ipfscluster.Component
}