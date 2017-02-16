package cluster

import (
	"github.com/Sirupsen/logrus"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

func multiaddrSplit(addr ma.Multiaddr) (peer.ID, ma.Multiaddr, error) {
	pid, err := addr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		logrus.WithError(err).WithField("address", addr).Error("invalid peer multiaddress")
		return "", nil, err
	}

	ipfs, _ := ma.NewMultiaddr("/ipfs/" + pid)
	decapAddr := addr.Decapsulate(ipfs)

	peerID, err := peer.IDB58Decode(pid)
	if err != nil {
		logrus.WithError(err).WithField("peerId", pid).Error("invalid peer ID in multiaddress")
		return "", nil, err
	}
	return peerID, decapAddr, nil
}
