// Package service contains functions to work with the Service type.
package service

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kakeetopius/qosm/internal/priority"
	"golang.org/x/sys/unix"
)

type IPProtocol uint

const (
	IPProtocolTCP IPProtocol = unix.IPPROTO_TCP
	IPProtocolUDP IPProtocol = unix.IPPROTO_UDP
)

type Service struct {
	Port     uint16
	Protocol IPProtocol
}

type ServiceRule struct {
	ID int
	Service
	Priority  priority.Priority
	CreatedAt time.Time
}

func (p IPProtocol) String() string {
	switch p {
	case IPProtocolTCP:
		return "tcp"
	case IPProtocolUDP:
		return "udp"
	}

	return ""
}

func (s Service) String() string {
	return fmt.Sprintf("%s/%v", s.Protocol.String(), s.Port)
}

// ServiceFromString returns a Service struct from a string of form service/portnumber eg tcp/80
func ServiceFromString(s string) (Service, error) {
	params := strings.Split(s, "/")
	if len(params) != 2 {
		return Service{}, fmt.Errorf("invalid service specification: %s. the format is protocol/port eg tcp/80", s)
	}

	service := Service{}
	switch params[0] {
	case "tcp":
		service.Protocol = IPProtocolTCP
	case "udp":
		service.Protocol = IPProtocolUDP
	default:
		return Service{}, fmt.Errorf("invalid service specification: %s. the format is protocol/port eg tcp/80", s)
	}

	portNum, err := strconv.Atoi(params[1])
	if err != nil {
		return Service{}, fmt.Errorf("invalid service specification: %s. the format is protocol/port eg tcp/80", s)
	}
	service.Port = uint16(portNum)

	return service, nil
}

func ServiceFromNftSetKey(key []byte) (Service, error) {
	if len(key) < 8 {
		return Service{}, fmt.Errorf("nft key has a wrong size. expected 8 bytes got: %v bytes", len(key))
	}
	proto := uint(key[0])

	port := binary.BigEndian.Uint16(key[4:])

	return Service{
		Protocol: IPProtocol(proto),
		Port:     port,
	}, nil
}

func (s Service) NFTSetKey() []byte {
	key := make([]byte, 8)

	// inet_proto (1 byte) padded to 4 bytes.
	key[0] = byte(s.Protocol)

	// inet_service padded to 4 bytes
	binary.BigEndian.PutUint16(key[4:], s.Port)

	return key
}
