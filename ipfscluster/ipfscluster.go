package ipfscluster

import (
	protocol "github.com/libp2p/go-libp2p-protocol"
)

// RPCProtocol is used to send libp2p messages between cluster peers
var RPCProtocol = protocol.ID("/ipfscluster/" + Version + "/rpc")

// Version is the current cluster version. Version alignment between
// components, apis and tools ensures compatibility among them.
var Version = "0.0.1"

// Commit is the current build commit of cluster. See Makefile
var Commit string
