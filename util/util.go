package util

import (
	"github.com/Sirupsen/logrus"
	"github.com/ipfs/ipfs-cluster/api"
	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// The copy functions below are used in calls to Cluste.multiRPC()
// func copyPIDsToIfaces(in []peer.ID) []interface{} {
// 	ifaces := make([]interface{}, len(in), len(in))
// 	for i := range in {
// 		ifaces[i] = &in[i]
// 	}
// 	return ifaces
// }

func CopyIDSerialsToIfaces(in []api.IDSerial) []interface{} {
	ifaces := make([]interface{}, len(in), len(in))
	for i := range in {
		ifaces[i] = &in[i]
	}
	return ifaces
}

func CopyPinInfoSerialToIfaces(in []api.PinInfoSerial) []interface{} {
	ifaces := make([]interface{}, len(in), len(in))
	for i := range in {
		ifaces[i] = &in[i]
	}
	return ifaces
}

func CopyPinInfoSerialSliceToIfaces(in [][]api.PinInfoSerial) []interface{} {
	ifaces := make([]interface{}, len(in), len(in))
	for i := range in {
		ifaces[i] = &in[i]
	}
	return ifaces
}

func CopyEmptyStructToIfaces(in []struct{}) []interface{} {
	ifaces := make([]interface{}, len(in), len(in))
	for i := range in {
		ifaces[i] = &in[i]
	}
	return ifaces
}

func MultiaddrSplit(addr ma.Multiaddr) (peer.ID, ma.Multiaddr, error) {
	pid, err := addr.ValueForProtocol(ma.P_IPFS)
	if err != nil {
		logrus.WithError(err).WithField("address", addr).Error("invalid peer multiaddress")
		return "", nil, err
	}

	ipfs, _ := ma.NewMultiaddr("/ipfs/" + pid)
	decapAddr := addr.Decapsulate(ipfs)

	peerID, err := peer.IDB58Decode(pid)
	if err != nil {
		logrus.WithError(err).WithField("peerId", pid).Error(err)
		return "", nil, err
	}
	return peerID, decapAddr, nil
}

func MultiaddrJoin(addr ma.Multiaddr, p peer.ID) ma.Multiaddr {
	pidAddr, err := ma.NewMultiaddr("/ipfs/" + peer.IDB58Encode(p))
	// let this break badly
	if err != nil {
		panic("called multiaddrJoin with bad peer!")
	}
	return addr.Encapsulate(pidAddr)
}

func PeersFromMultiaddrs(addrs []ma.Multiaddr) []peer.ID {
	var pids []peer.ID
	for _, addr := range addrs {
		pid, _, err := MultiaddrSplit(addr)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

// // connect to a peer ID.
// func connectToPeer(ctx context.Context, h host.Host, id peer.ID, addr ma.Multiaddr) error {
// 	err := h.Connect(ctx, peerstore.PeerInfo{
// 		ID:    id,
// 		Addrs: []ma.Multiaddr{addr},
// 	})
// 	return err
// }

// // return the local multiaddresses used to communicate to a peer.
// func localMultiaddrsTo(h host.Host, pid peer.ID) []ma.Multiaddr {
// 	var addrs []ma.Multiaddr
// 	conns := h.Network().ConnsToPeer(pid)
// 	logger.Debugf("conns to %s are: %s", pid, conns)
// 	for _, conn := range conns {
// 		addrs = append(addrs, multiaddrJoin(conn.LocalMultiaddr(), h.ID()))
// 	}
// 	return addrs
// }

// If we have connections open to that PID and they are using a different addr
// then we return the one we are using, otherwise the one provided
func GetRemoteMultiaddr(h host.Host, pid peer.ID, addr ma.Multiaddr) ma.Multiaddr {
	conns := h.Network().ConnsToPeer(pid)
	if len(conns) > 0 {
		return MultiaddrJoin(conns[0].RemoteMultiaddr(), pid)
	}
	return MultiaddrJoin(addr, pid)
}

func PinInfoSliceToSerial(pi []api.PinInfo) []api.PinInfoSerial {
	pis := make([]api.PinInfoSerial, len(pi), len(pi))
	for i, v := range pi {
		pis[i] = v.ToSerial()
	}
	return pis
}

func GlobalPinInfoSliceToSerial(gpi []api.GlobalPinInfo) []api.GlobalPinInfoSerial {
	gpis := make([]api.GlobalPinInfoSerial, len(gpi), len(gpi))
	for i, v := range gpi {
		gpis[i] = v.ToSerial()
	}
	return gpis
}

//func logError(fmtstr string, args ...interface{}) error {
//	msg := fmt.Sprintf(fmtstr, args...)
//	logrus.Error(msg)
//	return errors.New(msg)
//}
