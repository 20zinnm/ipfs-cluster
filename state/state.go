package state

import (
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
)

// State represents the shared state of the cluster and it is used by the Consensus component to keep track of objects which objects are pinned.
//
// This component should be thread safe.
type State interface {
	// Add adds a pin to the State
	Add(api.CidArg) error
	// Rm removes a pin from the State
	Rm(*cid.Cid) error
	// List lists all the pins in the state
	List() []api.CidArg
	// Has returns true if the state is holding information for a Cid
	Has(*cid.Cid) bool
	// Get returns the information attacthed to this pin
	Get(*cid.Cid) api.CidArg
}
