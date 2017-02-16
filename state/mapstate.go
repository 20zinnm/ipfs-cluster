package state

import (
	"sync"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
)

// MapVersion is the map state version.
//
// States with old versions should perform an upgrade before.
const MapVersion = 1

// mapState is a very simple database to store the state of the system using a Go map.
//
// It is thread safe.
type mapState struct {
	pinMux  sync.RWMutex
	PinMap  map[string]api.CidArgSerial
	Version int
}

// NewMap returns a very simple, thread-safe database to store the state of the system using a Go map.
func NewMap() State {
	return &mapState{
		PinMap: make(map[string]api.CidArgSerial),
	}
}

// Add adds a CidArg to the internal map.
func (st *mapState) Add(c api.CidArg) error {
	st.pinMux.Lock()
	defer st.pinMux.Unlock()
	st.PinMap[c.Cid.String()] = c.ToSerial()
	return nil
}

// Rm removes a Cid from the internal map.
func (st *mapState) Rm(c *cid.Cid) error {
	st.pinMux.Lock()
	defer st.pinMux.Unlock()
	delete(st.PinMap, c.String())
	return nil
}

// Get returns CidArg information for a CID.
func (st *mapState) Get(c *cid.Cid) api.CidArg {
	st.pinMux.RLock()
	defer st.pinMux.RUnlock()
	cargs, ok := st.PinMap[c.String()]
	if !ok { // make sure no panics
		return api.CidArg{}
	}
	return cargs.ToCidArg()
}

// Has returns true if the Cid belongs to the State.
func (st *mapState) Has(c *cid.Cid) bool {
	st.pinMux.RLock()
	defer st.pinMux.RUnlock()
	_, ok := st.PinMap[c.String()]
	return ok
}

// List provides the list of tracked CidArgs.
func (st *mapState) List() []api.CidArg {
	st.pinMux.RLock()
	defer st.pinMux.RUnlock()
	cids := make([]api.CidArg, 0, len(st.PinMap))
	for _, v := range st.PinMap {
		cids = append(cids, v.ToCidArg())
	}
	return cids
}
