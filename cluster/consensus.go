package cluster

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	rpc "github.com/hsanjuan/go-libp2p-gorpc"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/state"
	"github.com/ipfs/ipfs-cluster/util"
	libp2pconsensus "github.com/libp2p/go-libp2p-consensus"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	libp2praft "github.com/libp2p/go-libp2p-raft"
	ma "github.com/multiformats/go-multiaddr"
)

// LeaderTimeout specifies how long to wait before failing an operation
// because there is no leader
var LeaderTimeout = 15 * time.Second

// CommitRetries specifies how many times we retry a failed commit until
// we give up
var CommitRetries = 2

// Consensus handles the work of keeping a shared-state between
// the peers of an IPFS Cluster, as well as modifying that state and
// applying any updates in a thread-safe manner.
type Consensus struct {
	ctx context.Context

	host host.Host

	consensus libp2pconsensus.OpLogConsensus
	actor     libp2pconsensus.Actor
	baseOp    *LogOp
	raft      *Raft

	rpcClient *rpc.Client
	rpcReady  chan struct{}
	readyCh   chan struct{}

	shutdownLock sync.Mutex
	shutdown     bool
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
}

// NewConsensus builds a new ClusterConsensus component. The state
// is used to initialize the Consensus system, so any information in it
// is discarded.
func NewConsensus(clusterPeers []peer.ID, host host.Host, dataFolder string, state state.State) (*Consensus, error) {
	ctx := context.Background()
	op := &LogOp{
		ctx: context.Background(),
	}

	logrus.Info("starting Consensus and waiting for a leader")
	consensus := libp2praft.NewOpLog(state, op)
	raft, err := NewRaft(clusterPeers, host, dataFolder, consensus.FSM())
	if err != nil {
		return nil, err
	}
	actor := libp2praft.NewActor(raft.raft)
	consensus.SetActor(actor)

	cc := &Consensus{
		ctx:        ctx,
		host:       host,
		consensus:  consensus,
		actor:      actor,
		baseOp:     op,
		raft:       raft,
		shutdownCh: make(chan struct{}, 1),
		rpcReady:   make(chan struct{}, 1),
		readyCh:    make(chan struct{}, 1),
	}

	cc.run()
	return cc, nil
}

func (cc *Consensus) run() {
	cc.wg.Add(1)
	go func() {
		defer cc.wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cc.ctx = ctx
		cc.baseOp.ctx = ctx

		go cc.finishBootstrap()
		<-cc.shutdownCh
	}()
}

// WaitForSync waits for a leader and for the state to be up to date, then returns.
func (cc *Consensus) WaitForSync() error {
	leaderCtx, cancel := context.WithTimeout(cc.ctx, LeaderTimeout)
	defer cancel()
	err := cc.raft.WaitForLeader(leaderCtx)
	if err != nil {
		return errors.New("error waiting for leader: " + err.Error())
	}
	err = cc.raft.WaitForUpdates(cc.ctx)
	if err != nil {
		return errors.New("error waiting for libp2pconsensus updates: " + err.Error())
	}
	return nil
}

// waits until there is a libp2pconsensus leader and syncs the state to the tracker
func (cc *Consensus) finishBootstrap() {
	err := cc.WaitForSync()
	if err != nil {
		return
	}
	logrus.Info("consensus state is up to date")

	// While rpc is not ready we cannot perform a sync
	if cc.rpcClient == nil {
		select {
		case <-cc.ctx.Done():
			return
		case <-cc.rpcReady:
		}
	}

	st, err := cc.State()
	_ = st
	// only check sync if we have a state avoid error on new running clusters
	if err != nil {
		logrus.WithError(err).Debug("skipping state sync")
	} else {
		var pInfoSerial []api.PinInfoSerial
		cc.rpcClient.Go(
			"",
			"Cluster",
			"StateSync",
				struct{}{},
			&pInfoSerial,
			nil)
	}
	cc.readyCh <- struct{}{}
	logrus.Debug("libp2pconsensus ready")
}

// Shutdown stops the component so it will not process any
// more updates. The underlying libp2pconsensus is permanently
// shutdown, along with the libp2p transport.
func (cc *Consensus) Shutdown() error {
	cc.shutdownLock.Lock()
	defer cc.shutdownLock.Unlock()

	if cc.shutdown {
		logrus.Debug("already shutdown")
		return nil
	}

	logrus.Info("stopping consensus component")

	close(cc.rpcReady)
	cc.shutdownCh <- struct{}{}

	// Raft shutdown
	errMsgs := make([]string, 0)
	err := cc.raft.Snapshot()
	if err != nil {
		errMsgs = append(errMsgs, err.Error())
	}
	err = cc.raft.Shutdown()
	if err != nil {
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) > 0 {
		logrus.WithField("errorMessages", errMsgs).Error("consensus shutdown unsuccessful")
		return errors.New(strings.Join(errMsgs, ", "))
	}
	cc.wg.Wait()
	cc.shutdown = true
	return nil
}

// SetClient makes the component ready to perform RPC requets
func (cc *Consensus) SetClient(c *rpc.Client) {
	cc.rpcClient = c
	cc.baseOp.rpcClient = c
	cc.rpcReady <- struct{}{}
}

// Ready returns a channel which is signaled when the Consensus
// algorithm has finished bootstrapping and is ready to use
func (cc *Consensus) Ready() <-chan struct{} {
	return cc.readyCh
}

func (cc *Consensus) op(argi interface{}, t LogOpType) *LogOp {
	switch argi.(type) {
	case api.CidArg:
		return &LogOp{
			Cid:  argi.(api.CidArg).ToSerial(),
			Type: t,
		}
	case ma.Multiaddr:
		return &LogOp{
			Peer: api.MultiaddrToSerial(argi.(ma.Multiaddr)),
			Type: t,
		}
	default:
		panic("bad type")
	}
}

// returns true if the operation was redirected to the leader
func (cc *Consensus) redirectToLeader(method string, arg interface{}) (bool, error) {
	leader, err := cc.Leader()
	if err != nil {
		rctx, cancel := context.WithTimeout(cc.ctx, LeaderTimeout)
		defer cancel()
		err := cc.raft.WaitForLeader(rctx)
		if err != nil {
			return false, err
		}
	}
	if leader == cc.host.ID() {
		return false, nil
	}

	err = cc.rpcClient.Call(
		leader,
		"Cluster",
		method,
		arg,
		&struct{}{})
	return true, err
}

func (cc *Consensus) logOpCid(rpcOp string, opType LogOpType, carg api.CidArg) error {
	var finalErr error
	for i := 0; i < CommitRetries; i++ {
		logrus.WithField("attempt", i).Debug("trying to commit log operation")
		redirected, err := cc.redirectToLeader(
			rpcOp, carg.ToSerial())
		if err != nil {
			finalErr = err
			continue
		}

		if redirected {
			return nil
		}

		// It seems WE are the leader.

		op := cc.op(carg, opType)
		_, err = cc.consensus.CommitOp(op)
		if err != nil {
			// This means the op did not make it to the log
			finalErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		finalErr = nil
		break
	}
	if finalErr != nil {
		return finalErr
	}

	switch opType {
	case LogOpPin:
		logrus.WithField("cid", carg.Cid).Info("pin committed to global state")
	case LogOpUnpin:
		logrus.WithField("cid", carg.Cid).Info("unpin committed to global state")
	}
	return nil
}

// LogPin submits a Cid to the shared state of the cluster or forwards the operation to the leader if this is not it.
func (cc *Consensus) LogPin(c api.CidArg) error {
	return cc.logOpCid("ConsensusLogPin", LogOpPin, c)
}

// LogUnpin removes a Cid from the shared state of the cluster.
func (cc *Consensus) LogUnpin(c api.CidArg) error {
	return cc.logOpCid("ConsensusLogUnpin", LogOpUnpin, c)
}

// LogAddPeer submits a new peer to the shared state of the cluster. It will
// forward the operation to the leader if this is not it.
func (cc *Consensus) LogAddPeer(addr ma.Multiaddr) error {
	var finalErr error
	for i := 0; i < CommitRetries; i++ {
		logrus.WithField("attempt", i).Debug("trying to add peer")
		redirected, err := cc.redirectToLeader(
			"ConsensusLogAddPeer", api.MultiaddrToSerial(addr))
		if err != nil {
			finalErr = err
			continue
		}

		if redirected {
			return nil
		}

		// It seems WE are the leader.
		pid, _, err := util.MultiaddrSplit(addr)
		if err != nil {
			return err
		}

		// Create pin operation for the log
		op := cc.op(addr, LogOpAddPeer)
		_, err = cc.consensus.CommitOp(op)
		if err != nil {
			// This means the op did not make it to the log
			finalErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		err = cc.raft.AddPeer(peer.IDB58Encode(pid))
		if err != nil {
			finalErr = err
			continue
		}
		finalErr = nil
		break
	}
	if finalErr != nil {
		return finalErr
	}
	logrus.WithField("address", addr).Info("peer committed to global state: %s")
	return nil
}

// LogRmPeer removes a peer from the shared state of the cluster. It will
// forward the operation to the leader if this is not it.
func (cc *Consensus) LogRmPeer(pid peer.ID) error {
	var finalErr error
	for i := 0; i < CommitRetries; i++ {
		logrus.WithField("attempt", i).Debug("trying to remove peer")
		redirected, err := cc.redirectToLeader("ConsensusLogRmPeer", pid)
		if err != nil {
			finalErr = err
			continue
		}

		if redirected {
			return nil
		}

		// It seems WE are the leader.

		// Create pin operation for the log
		addr, err := ma.NewMultiaddr("/ipfs/" + peer.IDB58Encode(pid))
		if err != nil {
			return err
		}
		op := cc.op(addr, LogOpRmPeer)
		_, err = cc.consensus.CommitOp(op)
		if err != nil {
			// This means the op did not make it to the log
			finalErr = err
			continue
		}
		err = cc.raft.RemovePeer(peer.IDB58Encode(pid))
		if err != nil {
			finalErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		finalErr = nil
		break
	}
	if finalErr != nil {
		return finalErr
	}
	logrus.WithField("peerId", pid).Info("peer removed from global state")
	return nil
}

// State retrieves the current libp2pconsensus State. It may error
// if no State has been agreed upon or the state is not
// consistent. The returned State is the last agreed-upon
// State known by this node.
func (cc *Consensus) State() (state.State, error) {
	st, err := cc.consensus.GetLogHead()
	if err != nil {
		return nil, err
	}
	state, ok := st.(state.State)
	if !ok {
		return nil, errors.New("wrong state type")
	}
	return state, nil
}

// Leader returns the peerID of the Leader of the
// cluster. It returns an error when there is no leader.
func (cc *Consensus) Leader() (peer.ID, error) {
	raftactor := cc.actor.(*libp2praft.Actor)
	return raftactor.Leader()
}

// Rollback replaces the current agreed-upon
// state with the state provided. Only the libp2pconsensus leader
// can perform this operation.
func (cc *Consensus) Rollback(state state.State) error {
	return cc.consensus.Rollback(state)
}
