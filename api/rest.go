package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	rpc "github.com/hsanjuan/go-libp2p-gorpc"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/ipfs-cluster/config"
	peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"
)

// Server settings
var (
	// maximum duration before timing out read of the request
	RESTAPIServerReadTimeout = 5 * time.Second
	// maximum duration before timing out write of the response
	RESTAPIServerWriteTimeout = 10 * time.Second
	// server-side the amount of time a Keep-Alive connection will be
	// kept idle before being reused
	RESTAPIServerIdleTimeout = 60 * time.Second
)

// restAPI implements an API and aims to provides a RESTful HTTP API for Cluster.
type restAPI struct {
	ctx        context.Context
	apiAddr    ma.Multiaddr
	listenAddr string
	listenPort int
	rpcClient  *rpc.Client
	rpcReady   chan struct{}
	router     *mux.Router

	listener net.Listener
	server   *http.Server

	shutdownLock sync.Mutex
	shutdown     bool
	wg           sync.WaitGroup
}

type route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type peerAddBody struct {
	PeerMultiaddr string `json:"peer_multiaddress"`
}

type errorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e errorResp) Error() string {
	return e.Message
}

// NewREST creates a new object which is ready to be started.
func NewREST(cfg config.Config) (API, error) {
	ctx := context.Background()

	listenAddr, err := cfg.APIAddr.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil, err
	}
	listenPortStr, err := cfg.APIAddr.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return nil, err
	}
	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		return nil, err
	}

	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d",
		listenAddr, listenPort))
	if err != nil {
		return nil, err
	}

	router := mux.NewRouter().StrictSlash(true)
	s := &http.Server{
		ReadTimeout:  RESTAPIServerReadTimeout,
		WriteTimeout: RESTAPIServerWriteTimeout,
		//IdleTimeout:  RESTAPIServerIdleTimeout, // TODO: Go 1.8
		Handler: router,
	}
	s.SetKeepAlivesEnabled(true) // A reminder that this can be changed

	rapi := &restAPI{
		ctx:        ctx,
		apiAddr:    cfg.APIAddr,
		listenAddr: listenAddr,
		listenPort: listenPort,
		listener:   l,
		server:     s,
		rpcReady:   make(chan struct{}, 1),
	}

	for _, route := range rapi.routes() {
		router.
		Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc)
	}

	rapi.router = router
	rapi.run()
	return rapi, nil
}

func (rest *restAPI) routes() []route {
	return []route{
		{
			"ID",
			"GET",
			"/id",
			rest.idHandler,
		},

		{
			"Version",
			"GET",
			"/version",
			rest.versionHandler,
		},

		{
			"Peers",
			"GET",
			"/peers",
			rest.peerListHandler,
		},
		{
			"PeerAdd",
			"POST",
			"/peers",
			rest.peerAddHandler,
		},
		{
			"PeerRemove",
			"DELETE",
			"/peers/{peer}",
			rest.peerRemoveHandler,
		},

		{
			"Pins",
			"GET",
			"/pinlist",
			rest.pinListHandler,
		},

		{
			"StatusAll",
			"GET",
			"/pins",
			rest.statusAllHandler,
		},
		{
			"SyncAll",
			"POST",
			"/pins/sync",
			rest.syncAllHandler,
		},
		{
			"Status",
			"GET",
			"/pins/{hash}",
			rest.statusHandler,
		},
		{
			"Pin",
			"POST",
			"/pins/{hash}",
			rest.pinHandler,
		},
		{
			"Unpin",
			"DELETE",
			"/pins/{hash}",
			rest.unpinHandler,
		},
		{
			"Sync",
			"POST",
			"/pins/{hash}/sync",
			rest.syncHandler,
		},
		{
			"Recover",
			"POST",
			"/pins/{hash}/recover",
			rest.recoverHandler,
		},
	}
}

func (rest *restAPI) run() {
	rest.wg.Add(1)
	go func() {
		defer rest.wg.Done()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		rest.ctx = ctx

		<-rest.rpcReady

		logrus.WithField("address", rest.apiAddr).Info("started REST API")
		err := rest.server.Serve(rest.listener)
		if err != nil && !strings.Contains(err.Error(), "closed network connection") {
			logrus.WithError(err).Error("the network connection was closed")
		}
	}()
}

// Shutdown stops any API listeners.
func (rest *restAPI) Shutdown() error {
	rest.shutdownLock.Lock()
	defer rest.shutdownLock.Unlock()

	if rest.shutdown {
		logrus.Debug("already shutdown")
		return nil
	}

	logrus.Info("stopping Cluster API")

	close(rest.rpcReady)
	// Cancel any outstanding ops
	rest.server.SetKeepAlivesEnabled(false)
	rest.listener.Close()

	rest.wg.Wait()
	rest.shutdown = true
	return nil
}

// SetClient makes the component ready to perform RPC
// requests.
func (rest *restAPI) SetClient(c *rpc.Client) {
	rest.rpcClient = c
	rest.rpcReady <- struct{}{}
}

func (rest *restAPI) idHandler(w http.ResponseWriter, r *http.Request) {
	idSerial := IDSerial{}
	err := rest.rpcClient.Call("",
		"Cluster",
		"ID",
			struct{}{},
		&idSerial)

	sendResponse(w, err, idSerial)
}

func (rest *restAPI) versionHandler(w http.ResponseWriter, r *http.Request) {
	var v Version
	err := rest.rpcClient.Call("",
		"Cluster",
		"Version",
			struct{}{},
		&v)

	sendResponse(w, err, v)
}

func (rest *restAPI) peerListHandler(w http.ResponseWriter, r *http.Request) {
	var peersSerial []IDSerial
	err := rest.rpcClient.Call("",
		"Cluster",
		"Peers",
			struct{}{},
		&peersSerial)

	sendResponse(w, err, peersSerial)
}

func (rest *restAPI) peerAddHandler(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()

	var addInfo peerAddBody
	err := dec.Decode(&addInfo)
	if err != nil {
		sendErrorResponse(w, 400, "error decoding request body")
		return
	}

	mAddr, err := ma.NewMultiaddr(addInfo.PeerMultiaddr)
	if err != nil {
		sendErrorResponse(w, 400, "error decoding peer_multiaddress")
		return
	}

	var ids IDSerial
	err = rest.rpcClient.Call("",
		"Cluster",
		"PeerAdd",
		MultiaddrToSerial(mAddr),
		&ids)
	sendResponse(w, err, ids)
}

func (rest *restAPI) peerRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if p := parsePidOrError(w, r); p != "" {
		err := rest.rpcClient.Call("",
			"Cluster",
			"PeerRemove",
			p,
			&struct{}{})
		sendEmptyResponse(w, err)
	}
}

func (rest *restAPI) pinHandler(w http.ResponseWriter, r *http.Request) {
	if c := parseCidOrError(w, r); c.Cid != "" {
		err := rest.rpcClient.Call("",
			"Cluster",
			"Pin",
			c,
			&struct{}{})
		sendAcceptedResponse(w, err)
	}
}

func (rest *restAPI) unpinHandler(w http.ResponseWriter, r *http.Request) {
	if c := parseCidOrError(w, r); c.Cid != "" {
		err := rest.rpcClient.Call("",
			"Cluster",
			"Unpin",
			c,
			&struct{}{})
		sendAcceptedResponse(w, err)
	}
}

func (rest *restAPI) pinListHandler(w http.ResponseWriter, r *http.Request) {
	var pins []CidArgSerial
	err := rest.rpcClient.Call("",
		"Cluster",
		"PinList",
			struct{}{},
		&pins)
	sendResponse(w, err, pins)
}

func (rest *restAPI) statusAllHandler(w http.ResponseWriter, r *http.Request) {
	var pinInfos []GlobalPinInfoSerial
	err := rest.rpcClient.Call("",
		"Cluster",
		"StatusAll",
			struct{}{},
		&pinInfos)
	sendResponse(w, err, pinInfos)
}

func (rest *restAPI) statusHandler(w http.ResponseWriter, r *http.Request) {
	if c := parseCidOrError(w, r); c.Cid != "" {
		var pinInfo GlobalPinInfoSerial
		err := rest.rpcClient.Call("",
			"Cluster",
			"Status",
			c,
			&pinInfo)
		sendResponse(w, err, pinInfo)
	}
}

func (rest *restAPI) syncAllHandler(w http.ResponseWriter, r *http.Request) {
	var pinInfos []GlobalPinInfoSerial
	err := rest.rpcClient.Call("",
		"Cluster",
		"SyncAll",
			struct{}{},
		&pinInfos)
	sendResponse(w, err, pinInfos)
}

func (rest *restAPI) syncHandler(w http.ResponseWriter, r *http.Request) {
	if c := parseCidOrError(w, r); c.Cid != "" {
		var pinInfo GlobalPinInfoSerial
		err := rest.rpcClient.Call("",
			"Cluster",
			"Sync",
			c,
			&pinInfo)
		sendResponse(w, err, pinInfo)
	}
}

func (rest *restAPI) recoverHandler(w http.ResponseWriter, r *http.Request) {
	if c := parseCidOrError(w, r); c.Cid != "" {
		var pinInfo GlobalPinInfoSerial
		err := rest.rpcClient.Call("",
			"Cluster",
			"Recover",
			c,
			&pinInfo)
		sendResponse(w, err, pinInfo)
	}
}

func parseCidOrError(w http.ResponseWriter, r *http.Request) CidArgSerial {
	vars := mux.Vars(r)
	hash := vars["hash"]
	_, err := cid.Decode(hash)
	if err != nil {
		sendErrorResponse(w, 400, "error decoding Cid: "+err.Error())
		return CidArgSerial{Cid: ""}
	}
	return CidArgSerial{Cid: hash}
}

func parsePidOrError(w http.ResponseWriter, r *http.Request) peer.ID {
	vars := mux.Vars(r)
	idStr := vars["peer"]
	pid, err := peer.IDB58Decode(idStr)
	if err != nil {
		sendErrorResponse(w, 400, "error decoding Peer ID: "+err.Error())
		return ""
	}
	return pid
}

func sendResponse(w http.ResponseWriter, rpcErr error, resp interface{}) {
	if checkRPCErr(w, rpcErr) {
		sendJSONResponse(w, 200, resp)
	}
}

// checkRPCErr takes care of returning standard error responses if we
// pass an error to it. It returns true when everythings OK (no error
// was handled), or false otherwise.
func checkRPCErr(w http.ResponseWriter, err error) bool {
	if err != nil {
		sendErrorResponse(w, 500, err.Error())
		return false
	}
	return true
}

func sendEmptyResponse(w http.ResponseWriter, rpcErr error) {
	if checkRPCErr(w, rpcErr) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func sendAcceptedResponse(w http.ResponseWriter, rpcErr error) {
	if checkRPCErr(w, rpcErr) {
		w.WriteHeader(http.StatusAccepted)
	}
}

func sendJSONResponse(w http.ResponseWriter, code int, resp interface{}) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(err)
	}
}

func sendErrorResponse(w http.ResponseWriter, code int, msg string) {
	errorResp := errorResp{code, msg}
	logrus.WithFields(logrus.Fields{"code": code, "message": msg}).Info("sending error response")
	sendJSONResponse(w, code, errorResp)
}
