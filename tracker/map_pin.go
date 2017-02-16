package tracker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	rpc "github.com/hsanjuan/go-libp2p-gorpc"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/config"
	peer "github.com/libp2p/go-libp2p-peer"
)

// A Pin or Unpin operation will be considered failed
// if the Cid has stayed in Pinning or Unpinning state
// for longer than these values.
var (
	PinningTimeout   = 15 * time.Minute
	UnpinningTimeout = 10 * time.Second
)

// PinQueueSize specifies the maximum amount of pin operations waiting
// to be performed. If the queue is full, pins/unpins will be set to
// pinError/unpinError.
var PinQueueSize = 1024

var (
	errUnpinningTimeout = errors.New("unpinning operation is taking too long")
	errPinningTimeout   = errors.New("pinning operation is taking too long")
	errPinned           = errors.New("the item is unexpectedly pinned on IPFS")
	errUnpinned         = errors.New("the item is unexpectedly not pinned on IPFS")
)

// mapPinTracker is a PinTracker implementation which uses a Go map
// to store the status of the tracked Cids. This component is thread-safe.
type mapPinTracker struct {
	mux    sync.RWMutex
	status map[string]api.PinInfo

	ctx    context.Context
	cancel func()

	rpcClient *rpc.Client
	rpcReady  chan struct{}

	peerID  peer.ID
	pinCh   chan api.CidArg
	unpinCh chan api.CidArg

	shutdownLock sync.Mutex
	shutdown     bool
	wg           sync.WaitGroup
}

// NewMapPin returns a tracker which uses a Go map to store the status of the tracked Cids.
func NewMapPin(cfg config.Config) *mapPinTracker {
	ctx, cancel := context.WithCancel(context.Background())

	mpt := &mapPinTracker{
		ctx:      ctx,
		cancel:   cancel,
		status:   make(map[string]api.PinInfo),
		rpcReady: make(chan struct{}, 1),
		peerID:   cfg.ID,
		pinCh:    make(chan api.CidArg, PinQueueSize),
		unpinCh:  make(chan api.CidArg, PinQueueSize),
	}
	go mpt.pinWorker()
	go mpt.unpinWorker()
	return mpt
}

// reads the queue and makes pins to the IPFS daemon one by one
func (mpt *mapPinTracker) pinWorker() {
	for {
		select {
		case p := <-mpt.pinCh:
			mpt.pin(p)
		case <-mpt.ctx.Done():
			return
		}
	}
}

// reads the queue and makes unpin requests to the IPFS daemon
func (mpt *mapPinTracker) unpinWorker() {
	for {
		select {
		case p := <-mpt.unpinCh:
			mpt.unpin(p)
		case <-mpt.ctx.Done():
			return
		}
	}
}

// Shutdown finishes the services provided by the mapPinTracker and cancels
// any active context.
func (mpt *mapPinTracker) Shutdown() error {
	mpt.shutdownLock.Lock()
	defer mpt.shutdownLock.Unlock()

	if mpt.shutdown {
		logrus.Debug("already shutdown")
		return nil
	}

	logrus.Info("stopping mapPinTracker")
	mpt.cancel()
	close(mpt.rpcReady)
	mpt.wg.Wait()
	mpt.shutdown = true
	return nil
}

func (mpt *mapPinTracker) set(c *cid.Cid, s api.TrackerStatus) {
	mpt.mux.Lock()
	defer mpt.mux.Unlock()
	mpt.unsafeSet(c, s)
}

func (mpt *mapPinTracker) unsafeSet(c *cid.Cid, s api.TrackerStatus) {
	if s == api.TrackerStatusUnpinned {
		delete(mpt.status, c.String())
		return
	}

	mpt.status[c.String()] = api.PinInfo{
		Cid:    c,
		Peer:   mpt.peerID,
		Status: s,
		TS:     time.Now(),
		Error:  "",
	}
}

func (mpt *mapPinTracker) get(c *cid.Cid) api.PinInfo {
	mpt.mux.RLock()
	defer mpt.mux.RUnlock()
	return mpt.unsafeGet(c)
}

func (mpt *mapPinTracker) unsafeGet(c *cid.Cid) api.PinInfo {
	p, ok := mpt.status[c.String()]
	if !ok {
		return api.PinInfo{
			Cid:    c,
			Peer:   mpt.peerID,
			Status: api.TrackerStatusUnpinned,
			TS:     time.Now(),
			Error:  "",
		}
	}
	return p
}

// sets a Cid in error state
func (mpt *mapPinTracker) setError(c *cid.Cid, err error) {
	mpt.mux.Lock()
	defer mpt.mux.Unlock()
	mpt.unsafeSetError(c, err)
}

func (mpt *mapPinTracker) unsafeSetError(c *cid.Cid, err error) {
	p := mpt.unsafeGet(c)
	switch p.Status {
	case api.TrackerStatusPinned, api.TrackerStatusPinning, api.TrackerStatusPinError:
		mpt.status[c.String()] = api.PinInfo{
			Cid:    c,
			Peer:   mpt.peerID,
			Status: api.TrackerStatusPinError,
			TS:     time.Now(),
			Error:  err.Error(),
		}
	case api.TrackerStatusUnpinned, api.TrackerStatusUnpinning, api.TrackerStatusUnpinError:
		mpt.status[c.String()] = api.PinInfo{
			Cid:    c,
			Peer:   mpt.peerID,
			Status: api.TrackerStatusUnpinError,
			TS:     time.Now(),
			Error:  err.Error(),
		}
	}
}

func (mpt *mapPinTracker) isRemote(c api.CidArg) bool {
	if c.Everywhere {
		return false
	}

	for _, p := range c.Allocations {
		if p == mpt.peerID {
			return false
		}
	}
	return true
}

func (mpt *mapPinTracker) pin(c api.CidArg) error {
	mpt.set(c.Cid, api.TrackerStatusPinning)
	err := mpt.rpcClient.Call("",
		"Cluster",
		"IPFSPin",
		c.ToSerial(),
		&struct{}{})

	if err != nil {
		mpt.setError(c.Cid, err)
		return err
	}

	mpt.set(c.Cid, api.TrackerStatusPinned)
	return nil
}

func (mpt *mapPinTracker) unpin(c api.CidArg) error {
	err := mpt.rpcClient.Call("",
		"Cluster",
		"IPFSUnpin",
		c.ToSerial(),
		&struct{}{})

	if err != nil {
		mpt.setError(c.Cid, err)
		return err
	}
	mpt.set(c.Cid, api.TrackerStatusUnpinned)
	return nil
}

var ErrPinQueueFull = errors.New("pin queue is full")

// Track tells the mapPinTracker to start managing a Cid,
// possibly trigerring Pin operations on the IPFS daemon.
func (mpt *mapPinTracker) Track(c api.CidArg) error {
	if mpt.isRemote(c) {
		if mpt.get(c.Cid).Status == api.TrackerStatusPinned {
			mpt.unpin(c)
		}
		mpt.set(c.Cid, api.TrackerStatusRemote)
		return nil
	}

	mpt.set(c.Cid, api.TrackerStatusPinning)
	select {
	case mpt.pinCh <- c:
	default:
		mpt.setError(c.Cid, ErrPinQueueFull)
		logrus.WithError(ErrPinQueueFull).Error("pin queue is full")
		return ErrPinQueueFull
	}
	return nil
}

var ErrUnpinQueueFull = errors.New("unpin queue is full")

// Untrack tells the mapPinTracker to stop managing a Cid.
// If the Cid is pinned locally, it will be unpinned.
func (mpt *mapPinTracker) Untrack(c *cid.Cid) error {
	mpt.set(c, api.TrackerStatusUnpinning)
	select {
	case mpt.unpinCh <- api.CidArgCid(c):
	default:
		mpt.setError(c, ErrUnpinQueueFull)
		logrus.WithError(ErrUnpinQueueFull).Error("unpin queue is full")
		return ErrUnpinQueueFull
	}
	return nil
}

// Status returns information for a Cid tracked by this
// mapPinTracker.
func (mpt *mapPinTracker) Status(c *cid.Cid) api.PinInfo {
	return mpt.get(c)
}

// StatusAll returns information for all Cids tracked by this
// mapPinTracker.
func (mpt *mapPinTracker) StatusAll() []api.PinInfo {
	mpt.mux.Lock()
	defer mpt.mux.Unlock()
	pins := make([]api.PinInfo, 0, len(mpt.status))
	for _, v := range mpt.status {
		pins = append(pins, v)
	}
	return pins
}

// Sync verifies that the status of a Cid matches that of
// the IPFS daemon. If not, it will be transitioned
// to PinError or UnpinError.
//
// Sync returns the updated local status for the given Cid.
// Pins in error states can be recovered with Recover().
// An error is returned if we are unable to contact
// the IPFS daemon.
func (mpt *mapPinTracker) Sync(c *cid.Cid) (api.PinInfo, error) {
	var ips api.IPFSPinStatus
	err := mpt.rpcClient.Call("",
		"Cluster",
		"IPFSPinLsCid",
		api.CidArgCid(c).ToSerial(),
		&ips)
	if err != nil {
		mpt.setError(c, err)
		return mpt.get(c), err
	}
	return mpt.syncStatus(c, ips), nil
}

// SyncAll verifies that the statuses of all tracked Cids match the
// one reported by the IPFS daemon. If not, they will be transitioned
// to PinError or UnpinError.
//
// SyncAll returns the list of local status for all tracked Cids which
// were updated or have errors. Cids in error states can be recovered
// with Recover().
// An error is returned if we are unable to contact the IPFS daemon.
func (mpt *mapPinTracker) SyncAll() ([]api.PinInfo, error) {
	var ipsMap map[string]api.IPFSPinStatus
	var pInfos []api.PinInfo
	err := mpt.rpcClient.Call("",
		"Cluster",
		"IPFSPinLs",
		"recursive",
		&ipsMap)
	if err != nil {
		mpt.mux.Lock()
		for k := range mpt.status {
			c, _ := cid.Decode(k)
			mpt.unsafeSetError(c, err)
			pInfos = append(pInfos, mpt.unsafeGet(c))
		}
		mpt.mux.Unlock()
		return pInfos, err
	}

	status := mpt.StatusAll()
	for _, pInfoOrig := range status {
		var pInfoNew api.PinInfo
		c := pInfoOrig.Cid
		ips, ok := ipsMap[c.String()]
		if !ok {
			pInfoNew = mpt.syncStatus(c, api.IPFSPinStatusUnpinned)
		} else {
			pInfoNew = mpt.syncStatus(c, ips)
		}

		if pInfoOrig.Status != pInfoNew.Status ||
			pInfoNew.Status == api.TrackerStatusUnpinError ||
			pInfoNew.Status == api.TrackerStatusPinError {
			pInfos = append(pInfos, pInfoNew)
		}
	}
	return pInfos, nil
}

func (mpt *mapPinTracker) syncStatus(c *cid.Cid, ips api.IPFSPinStatus) api.PinInfo {
	p := mpt.get(c)
	if ips.IsPinned() {
		switch p.Status {
		case api.TrackerStatusPinned: // nothing
		case api.TrackerStatusPinning, api.TrackerStatusPinError:
			mpt.set(c, api.TrackerStatusPinned)
		case api.TrackerStatusUnpinning:
			if time.Since(p.TS) > UnpinningTimeout {
				mpt.setError(c, errUnpinningTimeout)
			}
		case api.TrackerStatusUnpinned:
			mpt.setError(c, errPinned)
		case api.TrackerStatusUnpinError: // nothing, keep error as it was
		default:                          //remote
		}
	} else {
		switch p.Status {
		case api.TrackerStatusPinned:

			mpt.setError(c, errUnpinned)
		case api.TrackerStatusPinError: // nothing, keep error as it was
		case api.TrackerStatusPinning:
			if time.Since(p.TS) > PinningTimeout {
				mpt.setError(c, errPinningTimeout)
			}
		case api.TrackerStatusUnpinning, api.TrackerStatusUnpinError:
			mpt.set(c, api.TrackerStatusUnpinned)
		case api.TrackerStatusUnpinned: // nothing
		default:                        // remote
		}
	}
	return mpt.get(c)
}

// Recover will re-track or re-untrack a Cid in error state,
// possibly retriggering an IPFS pinning operation and returning
// only when it is done. The pinning/unpinning operation happens
// synchronously, jumping the queues.
func (mpt *mapPinTracker) Recover(c *cid.Cid) (api.PinInfo, error) {
	p := mpt.get(c)
	if p.Status != api.TrackerStatusPinError &&
		p.Status != api.TrackerStatusUnpinError {
		return p, nil
	}
	logrus.WithField("cid", c).Info("recovering cid")
	var err error
	switch p.Status {
	case api.TrackerStatusPinError:
		err = mpt.pin(api.CidArg{Cid: c})
	case api.TrackerStatusUnpinError:
		err = mpt.unpin(api.CidArg{Cid: c})
	}
	if err != nil {
		logrus.WithError(err).WithField("cid", c).Error("error recovering a cid")
	}
	return mpt.get(c), err
}

// SetClient makes the mapPinTracker ready to perform RPC requests to
// other components.
func (mpt *mapPinTracker) SetClient(c *rpc.Client) {
	mpt.rpcClient = c
	mpt.rpcReady <- struct{}{}
}
