package nft

import (
	"errors"
	"log/slog"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

type NFTOpts struct {
	// CreateIfNotExists is used when looking up of the different nftables objects(table, sets, rules) to determine whether to create the objects if they are not present.
	CreateIfNotExists bool
	Logger            *slog.Logger
}

type IfaceIndex int

// NFT holds the nftables connection and all QOS table structures.
type NFT struct {
	conn     *nftables.Conn
	opts     NFTOpts
	QosTable QosTable
}

// QosTable represents the nftables table with all QOS chains and sets.
type QosTable struct {
	*nftables.Table

	OutputChain  QosChain
	ForwardChain QosChain

	IPSets      QosIPSets
	ServiceSets QosServiceSets

	// IfaceSet contains names of the network interfaces on which qos is enabled. Note: the key size must always be unix.IFNAMSIZ
	IfaceSet *nftables.Set
}

// QosChain holds the output and forward chains for QOS table.
type QosChain struct {
	*nftables.Chain
	Rules QosRules
}

// QosRules holds the nftables rules for high and low priority traffic.
type QosRules struct {
	HighPrioIP4Rule     *nftables.Rule // Marks packets destined for high-priority IPv4 addresses.
	LowPrioIP4Rule      *nftables.Rule // Marks packets destined for low-priority IPv4 addresses.
	HighPrioIP6Rule     *nftables.Rule // Marks packets destined for high-priority IPv6 addresses.
	LowPrioIP6Rule      *nftables.Rule // Marks packets destined for low-priority IPv6 addresses.
	HighPrioServiceRule *nftables.Rule // Marks packets destined for high-priority services (protocol/port).
	LowPrioServiceRule  *nftables.Rule // Marks packets destined for low-priority services (protocol/port).
}

// QosIPSets holds the nftables ip sets for high and low priority traffic.
type QosIPSets struct {
	// Note: the key sizes for both these sets must be 4 bytes ie size of the ipv4 address.

	HighPrioIP4Set *nftables.Set
	LowPrioIP4Set  *nftables.Set

	// Note: the key sizes for both these sets must be 16 bytes ie size of the ipv6 address.

	HighPrioIP6Set *nftables.Set
	LowPrioIP6Set  *nftables.Set
}

// QosServiceSets holds the nftables port sets for high and low priority traffic.
type QosServiceSets struct {
	// Note: the key sizes for both sets must be 8 bytes. The first 4 byte containing the ip protocol number (eg 6 for tcp) and last 4 bytes containing the port number

	HighPrioServiceSet *nftables.Set
	LowPrioServiceSet  *nftables.Set
}

const (
	TABLENAME        = "qosmtable"
	OUTPUTCHAINNAME  = "output"
	FORWARDCHAINNAME = "forward"
)

const (
	HIGHPRIOIP4RULE        = "high_prio_ip4_rule"
	HIGHPRIOIP6RULE        = "high_prio_ip6_rule"
	HIGHPRIOSERVICERULE    = "high_prio_service_rule"
	HIGHPRIOIP4SETNAME     = "high_prio_ipv4_addrs"
	HIGHPRIOIP6SETNAME     = "high_prio_ipv6_addrs"
	HIGHPRIOSERVICESETNAME = "high_prio_services"
)

const (
	LOWPRIOIP4RULE        = "low_prio_ip4_rule"
	LOWPRIOIP6RULE        = "low_prio_ip6_rule"
	LOWPRIOSERVICERULE    = "low_prio_service_rule"
	LOWPRIOIP4SETNAME     = "low_prio_ipv4_addrs"
	LOWPRIOIP6SETNAME     = "low_prio_ipv6_addrs"
	LOWPRIOSERVICESETNAME = "low_prio_services"
)

const IFACESETNAME = "qos_enabled_ifaces"

type chainParams struct {
	table       *nftables.Table // the table to lookup/add the chain to
	chainName   string
	hook        *nftables.ChainHook
	chainPolicy nftables.ChainPolicy
}

type ruleParams struct {
	table        *nftables.Table // the table to lookup/add the rule to
	chain        *nftables.Chain // the chain to lookup/add the rule to
	l3proto      uint
	ruleName     string
	keyExtractor []expr.Any    // nftables expressions that extract the lookup key for the targetSet from the packet into register 1.
	targetSet    *nftables.Set // Set against which the lookup is performed to determine whether the rule matches.
	ifaceSet     *nftables.Set // set containing names of the interfaces on which qos is enabled.
	mark         int           // mark to set on packets that the rule matches
}

type setParams struct {
	table          *nftables.Table // the table to lookup/add the set to.
	setName        string
	setType        nftables.SetDatatype
	concatentation bool // indicates that the setType is a concatentation eg ip proto . dport
	isInterval     bool
	keyEndianess   binaryutil.ByteOrder // optional endianess of the key
}

var (
	ErrNotFound      = errors.New("nft object not found")
	ErrTableNotFound = errors.New("qosm table not found")
	ErrChainNotFound = errors.New("qosm chains not found")
)

type ErrSetNotFound struct {
	Name string
}

func (e ErrSetNotFound) Error() string {
	return "nft set " + e.Name + " not found"
}

type ErrRuleNotFound struct {
	Name string
}

func (e ErrRuleNotFound) Error() string {
	return "nft chain " + e.Name + " not found"
}

type ErrSetElementExists struct {
	Element string
	SetName string
}

func (e ErrSetElementExists) Error() string {
	return "set element " + e.Element + " already exists in the set " + e.SetName
}

type ErrSetElementNotExists struct {
	Element string
	SetName string
}

func (e ErrSetElementNotExists) Error() string {
	return "set element " + e.Element + " does not exist in the set " + e.SetName
}

// DstIPv4Extractor returns a payload expression that loads the IPv4
// destination address from the network header into register 1.
func DstIPv4Extractor() []expr.Any {
	return []expr.Any{
		&expr.Payload{
			DestRegister: unix.NFT_REG_1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       16, // bytes from start of IP Layer (leads to dest IP)
			Len:          4,  // 4 bytes of the IPv4 addr
		},
	}
}

func DstIPv6Extractor() []expr.Any {
	return []expr.Any{
		&expr.Payload{
			DestRegister: unix.NFT_REG_1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       24, // Destination IPv6 address
			Len:          16, // 128-bit IPv6 address
		},
	}
}

// DstProtoPortExtractor returns payload expressions that load the concatentation of layer 4 protocol and transport-layer
// destination port (TCP or UDP) from the packet into register 1.
func DstProtoPortExtractor() []expr.Any {
	return []expr.Any{
		// Load layer 4 protocol into reg 1
		// Reg 1 is 16bytes divided into 4 smaller registers (4bytes each)
		&expr.Meta{
			Key:      expr.MetaKeyL4PROTO,
			Register: unix.NFT_REG32_00, // loads into the first sub-register inside register 1
		},
		&expr.Payload{
			DestRegister: unix.NFT_REG32_01, // loads into the second sub-register inside register 1
			Base:         expr.PayloadBaseTransportHeader,
			Offset:       2, // Destination port is after the 2-byte source port.
			Len:          2, // 16-bit TCP/UDP destination port.
		},
	}
}
