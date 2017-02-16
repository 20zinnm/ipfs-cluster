package state

import (
	"testing"

	cid "github.com/ipfs/go-cid"
	peer "github.com/libp2p/go-libp2p-peer"

	"github.com/ipfs/ipfs-cluster/api"
)

var testCid1, _ = cid.Decode("QmP63DkAFEnDYNjDYBpyNDfttu1fvUw99x1brscPzpqmmq")
var testPeerID1, _ = peer.IDB58Decode("QmXZrtE5jQwXNqCJMfHUTQkvhQ4ZAnqMnmzFMJfLewuabc")

var c = api.CidArg{
	Cid:         testCid1,
	Allocations: []peer.ID{testPeerID1},
	Everywhere:  false,
}

func TestAdd(t *testing.T) {
	ms := NewMapState()
	ms.Add(c)
	if !ms.Has(c.Cid) {
		t.Error("should have added it")
	}
}

func TestRm(t *testing.T) {
	ms := NewMapState()
	ms.Add(c)
	ms.Rm(c.Cid)
	if ms.Has(c.Cid) {
		t.Error("should have removed it")
	}
}

func TestGet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("paniced")
		}
	}()
	ms := NewMapState()
	ms.Add(c)
	get := ms.Get(c.Cid)
	if get.Cid.String() != c.Cid.String() ||
		get.Allocations[0] != c.Allocations[0] ||
		get.Everywhere != c.Everywhere {
		t.Error("returned something different")
	}
}

func TestList(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("paniced")
		}
	}()
	ms := NewMapState()
	ms.Add(c)
	list := ms.List()
	if list[0].Cid.String() != c.Cid.String() ||
		list[0].Allocations[0] != c.Allocations[0] ||
		list[0].Everywhere != c.Everywhere {
		t.Error("returned something different")
	}
}
