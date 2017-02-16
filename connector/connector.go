package connector

import (
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/ipfscluster"
)

// IPFSConnector is a component which allows cluster to interact with
// an IPFS daemon. This is a base component.
type IPFSConnector interface {
	ipfscluster.Component
	ID() (api.IPFSID, error)
	Pin(*cid.Cid) error
	Unpin(*cid.Cid) error
	PinLsCid(*cid.Cid) (api.IPFSPinStatus, error)
	PinLs(typeFilter string) (map[string]api.IPFSPinStatus, error)
}
