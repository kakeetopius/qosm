// Package daemon contains code to manage and talk to the qosm daemon that is responsible for carrying out privilidged tasks eg adding firewall rules and the htb qdisc on the interface
package daemon

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/protobuf"
	"github.com/kakeetopius/qosm/internal/service"
	"github.com/kakeetopius/qosm/internal/util"
	"google.golang.org/protobuf/proto"
)

type Options struct {
	SocketPath string
	Debug      bool
}

type Daemon struct {
	Options
	Classifier *nft.NFT
	TcConn     *tc.Tc
	Logger     *slog.Logger
}

func New(opts Options) (*Daemon, error) {
	if opts.SocketPath == "" {
		return nil, fmt.Errorf("no socket provided")
	}

	// clear old socket if any
	if err := os.Remove(opts.SocketPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(opts.SocketPath), 0o755); err != nil {
		return nil, err
	}

	handlerOpts := slog.HandlerOptions{}
	if opts.Debug {
		handlerOpts.Level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &handlerOpts))

	nftClassifier, err := nft.NewNFTCtx(nft.NFTOpts{
		CreateIfNotExists: true,
		Logger:            logger,
	})
	if err != nil {
		return nil, err
	}

	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return nil, err
	}

	return &Daemon{
		Options:    opts,
		Classifier: &nftClassifier,
		TcConn:     tcnl,
		Logger:     logger,
	}, nil
}

func (d *Daemon) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sock, err := net.Listen("unix", d.SocketPath)
	if err != nil {
		return fmt.Errorf("error creating socket: %w", err)
	}
	err = os.Chmod(d.SocketPath, 0o777)
	if err != nil {
		return err
	}

	go func() {
		util.AwaitSignal(ctx)
		d.cleanUp()
		os.Exit(1)
	}()

	fmt.Println("Waiting for connections.......")
	for {
		conn, err := sock.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		fmt.Println("Got connection from: ", conn.RemoteAddr())
		err = d.handleConnection(conn)
		if err != nil {
			log.Println(err)
		}
	}
}

func (d *Daemon) cleanUp() {
	os.Remove(d.SocketPath)
	d.TcConn.Close()
}

func (d *Daemon) handleConnection(conn net.Conn) error {
	defer conn.Close()

	requestLenBuf := make([]byte, 4)
	_, err := io.ReadFull(conn, requestLenBuf)
	if err != nil {
		return fmt.Errorf("error reading from client: %v", err)
	}
	requestLen := binary.BigEndian.Uint32(requestLenBuf)

	requestBuf := make([]byte, requestLen)
	_, err = io.ReadFull(conn, requestBuf)
	if err != nil {
		return fmt.Errorf("error reading from client: %v", err)
	}

	var request protobuf.Request
	err = proto.Unmarshal(requestBuf, &request)
	if err != nil {
		return fmt.Errorf("error reading from client: %v", err)
	}
	command := request.WhichCommand()

	switch command {
	case protobuf.Request_Command_not_set_case:
		err = fmt.Errorf("no command given")
	case protobuf.Request_AddHosts_case:
		err = d.handleAddHostsRequest(&request)
	case protobuf.Request_DeleteHosts_case:
		err = d.handleDeleteHostsRequest(&request)
	case protobuf.Request_AddServices_case:
		err = d.handleAddServicesRequest(&request)
	case protobuf.Request_DeleteServices_case:
		err = d.handleDeleteServicesRequest(&request)
	case protobuf.Request_EnableIfaces_case:
		err = d.handleEnableIfaceRequest(&request)
	case protobuf.Request_DisableIfaces_case:
		err = d.handleDisableIfaceRequest(&request)
	case protobuf.Request_FlushRules_case:
		err = d.handleFlushRulesRequest(&request)
	default:
		err = fmt.Errorf("unknown request")
	}

	if err != nil {
		return sendErrMsg(conn, err.Error())
	}
	return sendSuccesMsg(conn)
}

func (d *Daemon) handleAddHostsRequest(request *protobuf.Request) error {
	req := request.GetAddHosts()
	if req == nil {
		return nil
	}

	ipsAsSlices := req.GetIps()
	ipsAsPrefixes := util.IPPrefixesFromProtobufIPs(ipsAsSlices)

	prio := req.GetPriority()
	priority := priority.Priority(prio)

	err := d.Classifier.AddIPsToPriority(ipsAsPrefixes, priority)
	if err != nil {
		return err
	}

	return nil
}

func (d *Daemon) handleDeleteHostsRequest(request *protobuf.Request) error {
	req := request.GetDeleteHosts()
	if req == nil {
		return nil
	}

	ipsAsSlices := req.GetIps()
	ipsAsPrefixes := util.IPPrefixesFromProtobufIPs(ipsAsSlices)

	prio := req.GetPriority()
	priority := priority.Priority(prio)

	err := d.Classifier.DeleteIPsFromPriority(ipsAsPrefixes, priority)
	if err != nil {
		return err
	}

	return nil
}

func (d *Daemon) handleAddServicesRequest(request *protobuf.Request) error {
	req := request.GetAddServices()
	if req == nil {
		return nil
	}

	servsInReq := req.GetServices()
	services := make([]service.Service, 0, len(servsInReq))
	for _, serv := range servsInReq {
		services = append(services, service.ServiceFromProtobufService(serv))
	}

	prio := req.GetPriority()
	priority := priority.Priority(prio)

	err := d.Classifier.AddServicesToPriority(services, priority)
	if err != nil {
		return err
	}

	return nil
}

func (d *Daemon) handleDeleteServicesRequest(request *protobuf.Request) error {
	req := request.GetDeleteServices()
	if req == nil {
		return nil
	}

	servsInReq := req.GetServices()
	services := make([]service.Service, 0, len(servsInReq))
	for _, serv := range servsInReq {
		port := serv.GetPort()
		proto := serv.GetProtocol()
		services = append(services, service.ServiceFrom(uint16(port), service.IPProtocol(proto)))
	}

	prio := req.GetPriority()
	priority := priority.Priority(prio)

	err := d.Classifier.DeleteServicesFromPriority(services, priority)
	if err != nil {
		return err
	}

	return nil
}

func (d *Daemon) handleEnableIfaceRequest(request *protobuf.Request) error {
	req := request.GetEnableIfaces()
	if req == nil {
		return nil
	}

	ifaces := req.GetIfaces()
	for _, iface := range ifaces {
		ifIndex := iface.GetIfindex()
		ifName := iface.GetName()
		rate := iface.GetRate()

		err := htb.InitHTBOnIface(d.TcConn, int(ifIndex), rate, d.Logger)
		if err != nil && !errors.Is(err, htb.ErrQdisExists) {
			return err
		}

		err = d.Classifier.AddIfaces([]string{ifName})
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Daemon) handleDisableIfaceRequest(request *protobuf.Request) error {
	req := request.GetDisableIfaces()
	if req == nil {
		return nil
	}

	ifaces := req.GetIfaces()
	for _, iface := range ifaces {
		ifIndex := iface.GetIfindex()
		ifName := iface.GetName()

		err := htb.FlushQdiscFromIface(d.TcConn, int(ifIndex))
		if err != nil && !errors.Is(err, htb.ErrQdiscNotFound) {
			return err
		}

		err = d.Classifier.DeleteIfaces([]string{ifName})
		var errNotExists nft.ErrSetElementNotExists
		if err != nil && !errors.As(err, &errNotExists) {
			return err
		}
	}

	return nil
}

func (d *Daemon) handleFlushRulesRequest(request *protobuf.Request) (err error) {
	req := request.GetFlushRules()
	if req == nil {
		return nil
	}

	return d.Classifier.FlushAllRules()
}

func sendErrMsg(conn net.Conn, msg string) error {
	errorResponse := protobuf.Error_builder{
		ErrorMessage: &msg,
	}.Build()

	response := protobuf.Response_builder{
		Error: errorResponse,
	}.Build()

	return sendToClient(conn, response)
}

func sendSuccesMsg(conn net.Conn) error {
	successResp := protobuf.Success_builder{}.Build()
	response := protobuf.Response_builder{
		SuccessOp: successResp,
	}.Build()

	return sendToClient(conn, response)
}

func sendToClient(conn net.Conn, m proto.Message) error {
	respBytes, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	respLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(respLenBytes, uint32(len(respBytes)))

	_, err = conn.Write(respLenBytes)
	if err != nil {
		return fmt.Errorf("error sending data to client: %w", err)
	}

	_, err = conn.Write(respBytes)
	if err != nil {
		return fmt.Errorf("error sending data to client: %w", err)
	}

	return nil
}
