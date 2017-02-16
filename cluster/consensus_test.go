package cluster

import (
	"context"
	"os"
	"testing"
	"time"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/api"
	"github.com/ipfs/ipfs-cluster/test"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/ipfs/ipfs-cluster/util"
)

func cleanRaft() {
	os.RemoveAll(test.TestingConfig().ConsensusDataFolder)
}

func testingConsensus(t *testing.T) *Consensus {
	//logging.SetDebugLogging()
	cfg := test.TestingConfig()
	ctx := context.Background()
	h, err := util.MakeHost(ctx, cfg)
	if err != nil {
		t.Fatal("cannot create host:", err)
	}
	st := mapstate.NewMapState()
	cc, err := NewConsensus([]peer.ID{cfg.ID}, h, cfg.ConsensusDataFolder, st)
	if err != nil {
		t.Fatal("cannot create Consensus:", err)
	}
	cc.SetClient(test.NewMockRPCClient(t))
	<-cc.Ready()
	return cc
}

func TestShutdownConsensus(t *testing.T) {
	// Bring it up twice to make sure shutdown cleans up properly
	// but also to make sure raft comes up ok when re-initialized
	defer cleanRaft()
	cc := testingConsensus(t)
	err := cc.Shutdown()
	if err != nil {
		t.Fatal("Consensus cannot shutdown:", err)
	}
	cc.Shutdown()
	cc = testingConsensus(t)
	err = cc.Shutdown()
	if err != nil {
		t.Fatal("Consensus cannot shutdown:", err)
	}
}

func TestConsensusPin(t *testing.T) {
	cc := testingConsensus(t)
	defer cleanRaft() // Remember defer runs in LIFO order
	defer cc.Shutdown()

	c, _ := cid.Decode(test.TestCid1)
	err := cc.LogPin(api.CidArg{Cid: c, Everywhere: true})
	if err != nil {
		t.Error("the operation did not make it to the log:", err)
	}

	time.Sleep(250 * time.Millisecond)
	st, err := cc.State()
	if err != nil {
		t.Fatal("error gettinng state:", err)
	}

	pins := st.List()
	if len(pins) != 1 || pins[0].Cid.String() != test.TestCid1 {
		t.Error("the added pin should be in the state")
	}
}

func TestConsensusUnpin(t *testing.T) {
	cc := testingConsensus(t)
	defer cleanRaft()
	defer cc.Shutdown()

	c, _ := cid.Decode(test.TestCid2)
	err := cc.LogUnpin(api.CidArgCid(c))
	if err != nil {
		t.Error("the operation did not make it to the log:", err)
	}
}

func TestConsensusLeader(t *testing.T) {
	cc := testingConsensus(t)
	cfg := testingConfig()
	pID := cfg.ID
	defer cleanRaft()
	defer cc.Shutdown()
	l, err := cc.Leader()
	if err != nil {
		t.Fatal("No leader:", err)
	}

	if l != pID {
		t.Errorf("expected %s but the leader appears as %s", pID, l)
	}
}
