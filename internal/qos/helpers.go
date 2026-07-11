package qos

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/protobuf"
	"github.com/kakeetopius/qosm/internal/service"
	"github.com/kakeetopius/qosm/internal/util"
	"github.com/mdlayher/ethtool"
	"google.golang.org/protobuf/proto"
)

func (m *QoSManager) sendAddHostsRequest(ips []netip.Prefix, prio priority.Priority) error {
	protoIPs := util.IPPrefixesToProtobufIPs(ips)
	request := protobuf.Request_builder{
		AddHosts: protobuf.AddHosts_builder{
			Ips:      protoIPs,
			Priority: prio.ToProtobufPriority(),
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendDeleteHostsRequest(ips []netip.Prefix, prio priority.Priority) error {
	protoIPs := util.IPPrefixesToProtobufIPs(ips)
	request := protobuf.Request_builder{
		DeleteHosts: protobuf.DeleteHosts_builder{
			Ips:      protoIPs,
			Priority: prio.ToProtobufPriority(),
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendAddServicesRequest(servs []service.Service, prio priority.Priority) error {
	protoServices := make([]*protobuf.Service, 0, len(servs))
	for _, s := range servs {
		protoServices = append(protoServices, s.ToProtobufService())
	}
	request := protobuf.Request_builder{
		AddServices: protobuf.AddServices_builder{
			Services: protoServices,
			Priority: prio.ToProtobufPriority(),
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendDeleteServiceRequest(servs []service.Service, prio priority.Priority) error {
	protoServices := make([]*protobuf.Service, 0, len(servs))
	for _, s := range servs {
		protoServices = append(protoServices, s.ToProtobufService())
	}
	request := protobuf.Request_builder{
		DeleteServices: protobuf.DeleteServices_builder{
			Services: protoServices,
			Priority: prio.ToProtobufPriority(),
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendEnableIfaceRequest(ifName string, ifIndex int) error {
	request := protobuf.Request_builder{
		EnableIfaces: protobuf.EnableIfaces_builder{
			Ifaces: []*protobuf.Interface{util.GetProtobufInterface(ifName, int32(ifIndex))},
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendDisableIfaceRequest(ifName string, ifIndex int) error {
	request := protobuf.Request_builder{
		DisableIfaces: protobuf.DisableIfaces_builder{
			Ifaces: []*protobuf.Interface{util.GetProtobufInterface(ifName, int32(ifIndex))},
		}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendFlushAllRulesRequest() error {
	request := protobuf.Request_builder{
		FlushRules: protobuf.FlushRules_builder{}.Build(),
	}.Build()

	return m.sendRequestToDaemon(request)
}

func (m *QoSManager) sendRequestToDaemon(message proto.Message) error {
	reqBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}
	dataLen := make([]byte, 4)
	binary.BigEndian.PutUint32(dataLen, uint32(len(reqBytes)))

	conn, err := net.Dial("unix", m.DaemonSock)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to dial the daemon. Is the daemon running? If not start it with 'sudo qosm daemon run'")
		}
		return fmt.Errorf("failed to dial daemon: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(dataLen)
	if err != nil {
		return fmt.Errorf("error sending data to daemon: %w", err)
	}

	_, err = conn.Write(reqBytes)
	if err != nil {
		return fmt.Errorf("error sending data to daemon: %w", err)
	}

	respLenBytes := make([]byte, 4)
	_, err = io.ReadFull(conn, respLenBytes)
	if err != nil {
		return fmt.Errorf("error receiving data from daemon: %w", err)
	}
	responseLen := binary.BigEndian.Uint32(respLenBytes)

	respBytes := make([]byte, responseLen)
	_, err = io.ReadFull(conn, respBytes)
	if err != nil {
		return fmt.Errorf("error receiving data from daemon: %w", err)
	}

	var response protobuf.Response

	err = proto.Unmarshal(respBytes, &response)
	if err != nil {
		return fmt.Errorf("error decoding data from the daemon: %w", err)
	}

	whichResponse := response.WhichResponse()
	switch whichResponse {
	case protobuf.Response_Error_case:
		errResp := response.GetError()
		return fmt.Errorf("daemon returned error: %s", errResp.GetErrorMessage())
	case protobuf.Response_SuccessOp_case:
		return nil
	case protobuf.Response_Response_not_set_case:
		return nil
	}

	return nil
}

func getInterfaceSpeed(ifName string) (uint32, error) {
	client, err := ethtool.New()
	if err != nil {
		return 0, err
	}

	linkMode, err := client.LinkMode(ethtool.Interface{Name: ifName})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) { // returned if the interface is not an ethernet interface.
			return 0, nil
		}
		return 0, err
	}

	speed := linkMode.SpeedMegabits
	if speed == math.MaxUint32 { // returned if the interface has speed of -1 meaning speed is not known to kernel
		speed = 0
	}

	return uint32(speed), nil
}

func addRuleSuccessLog(dbCon *sql.DB, target string, priority string) error {
	return db.AddLog(
		dbCon,
		db.Log{
			EventType:   "RULE",
			Description: fmt.Sprintf("added %s to %s priority", target, strings.ToUpper(priority)),
		},
	)
}

func addRuleDeletedLog(dbCon *sql.DB, target string, priority string) error {
	return db.AddLog(
		dbCon,
		db.Log{
			EventType:   "RULE",
			Description: fmt.Sprintf("deleted %s prioriy rule for %s", strings.ToUpper(priority), target),
		},
	)
}

func addTCEnabledLog(dbCon *sql.DB, iface string) error {
	return db.AddLog(
		dbCon,
		db.Log{
			EventType:   "TC",
			Description: fmt.Sprintf("enabled traffic control on interface %s", iface),
		},
	)
}

func addTCDisabledLog(dbCon *sql.DB, iface string) error {
	return db.AddLog(
		dbCon,
		db.Log{
			EventType:   "TC",
			Description: fmt.Sprintf("disabled traffic control on interface %s", iface),
		},
	)
}

func ipSliceToString(ips []net.IP) string {
	if len(ips) == 0 {
		return ""
	}

	stringBuilder := strings.Builder{}
	for i, ip := range ips {
		stringBuilder.WriteString(ip.String())
		if i != len(ips)-1 {
			stringBuilder.WriteString(", ")
		}
	}

	return stringBuilder.String()
}

func joinIPAndDomainRules(ipRules []db.IPRule, domainRules []db.DomainRule) []Rule {
	allRules := make([]Rule, 0, len(ipRules)+len(domainRules))
	for _, rule := range ipRules {
		allRules = append(allRules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.IP,
			Type:      "ip",
			CreatedAt: rule.CreatedAt,
		})
	}

	for _, rule := range domainRules {
		allRules = append(allRules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.DomainName,
			Type:      "domain",
			CreatedAt: rule.CreatedAt,
		})
	}

	return allRules
}
