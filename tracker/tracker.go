package tracker

import (
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/ipfscluster"
)

// PinTracker represents a component which tracks the status of
// the pins in this cluster and ensures they are in sync with the
// IPFS daemon. This component should be thread safe.
type PinTracker interface {
	ipfscluster.Component
	// Track tells the tracker that a Cid is now under its supervision
	// The tracker may decide to perform an IPFS pin.
	Track(api.CidArg) error
	// Untrack tells the tracker that a Cid is to be forgotten. The tracker
	// may perform an IPFS unpin operation.
	Untrack(*cid.Cid) error
	// StatusAll returns the list of pins with their local status.
	StatusAll() []api.PinInfo
	// Status returns the local status of a given Cid.
	Status(*cid.Cid) api.PinInfo
	// SyncAll makes sure that all tracked Cids reflect the real IPFS status.
	// It returns the list of pins which were updated by the call.
	SyncAll() ([]api.PinInfo, error)
	// Sync makes sure that the Cid status reflect the real IPFS status.
	// It returns the local status of the Cid.
	Sync(*cid.Cid) (api.PinInfo, error)
	// Recover retriggers a Pin/Unpin operation in Cids with error status.
	Recover(*cid.Cid) (api.PinInfo, error)
}
