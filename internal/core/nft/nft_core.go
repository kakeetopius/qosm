// Package nft contains packet filtering for packets entering tc classes.
package nft

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"os"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/util"
	"golang.org/x/sys/unix"
)

func NewNFTCtx(opts NFTOpts) (NFT, error) {
	nftConn, err := nftables.New()
	if err != nil {
		return NFT{}, err
	}
	nft := NFT{
		conn: nftConn,
		opts: opts,
	}

	err = nft.initTable()
	if err != nil {
		return NFT{}, err
	}

	err = nft.initSets()
	if err != nil {
		return NFT{}, err
	}

	err = nft.initChains()
	if err != nil {
		return NFT{}, err
	}

	return nft, nil
}

// DeleteTable removes the qosm nftables table from the system.
func DeleteTable() error {
	conn, err := nftables.New()
	if err != nil {
		return err
	}

	tables, err := conn.ListTables()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if table.Name == TABLENAME {
			conn.DelTable(table)
			return conn.Flush()
		}
	}

	return ErrTableNotFound
}

func (c *NFT) initTable() error {
	qosTable, err := lookupQosTable(c.conn, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.Table = qosTable
	return nil
}

func (c *NFT) initChains() error {
	outputChain, err := lookupQosChain(c.conn, chainParams{
		table:       c.QosTable.Table,
		chainName:   OUTPUTCHAINNAME,
		hook:        nftables.ChainHookOutput,
		chainPolicy: nftables.ChainPolicyAccept,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.initRules(&outputChain)
	c.QosTable.OutputChain = outputChain

	forwardChain, err := lookupQosChain(c.conn, chainParams{
		table:       c.QosTable.Table,
		chainName:   FORWARDCHAINNAME,
		hook:        nftables.ChainHookForward,
		chainPolicy: nftables.ChainPolicyAccept,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.initRules(&forwardChain)
	c.QosTable.ForwardChain = forwardChain
	return nil
}

func (c *NFT) initRules(chain *QosChain) error {
	// Order is important here.
	// Rule 1: checks if the dst ipv4 is in high priority ip set and if so marks the packet with highprio mark and returns from the chain
	// Rule 2: checks if the dst ipv6 is in high priority ip set and if so marks the packet with highprio mark and returns from the chain
	// Rule 3: checks if the dst port and l4 protocol is in high priority port set and if so marks the packet with highprio mark and returns from the chain
	// Rule 4: checks if the dst ipv4 is in low priority ip set and if so marks the packet with lowprio mark and returns from the chain
	// Rule 5: checks if the dst ipv6 is in low priority ip set and if so marks the packet with lowprio mark and returns from the chain
	// Rule 6: checks if the dst port  and l4 protocol is in low priority port set and if so marks the packet with lowprio mark and returns from the chain

	highPrioIP4Rule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		ruleName:     HIGHPRIOIP4RULE,
		l3proto:      unix.NFPROTO_IPV4,
		keyExtractor: DstIPv4Extractor(),
		targetSet:    c.QosTable.IPSets.HighPrioIP4Set,
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYHIGH),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.HighPrioIP4Rule = highPrioIP4Rule

	highPrioIP6Rule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		ruleName:     HIGHPRIOIP6RULE,
		l3proto:      unix.NFPROTO_IPV6,
		keyExtractor: DstIPv6Extractor(),
		targetSet:    c.QosTable.IPSets.HighPrioIP6Set,
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYHIGH),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.HighPrioIP6Rule = highPrioIP6Rule

	highPrioServiceRule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		ruleName:     HIGHPRIOSERVICERULE,
		targetSet:    c.QosTable.ServiceSets.HighPrioServiceSet,
		keyExtractor: DstProtoPortExtractor(),
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYHIGH),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.HighPrioServiceRule = highPrioServiceRule

	lowPrioIP4Rule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		l3proto:      unix.NFPROTO_IPV4,
		ruleName:     LOWPRIOIP4RULE,
		targetSet:    c.QosTable.IPSets.LowPrioIP4Set,
		keyExtractor: DstIPv4Extractor(),
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYLOW),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.LowPrioIP4Rule = lowPrioIP4Rule

	lowPrioIP6Rule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		l3proto:      unix.NFPROTO_IPV6,
		ruleName:     LOWPRIOIP6RULE,
		targetSet:    c.QosTable.IPSets.LowPrioIP6Set,
		keyExtractor: DstIPv6Extractor(),
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYLOW),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.LowPrioIP6Rule = lowPrioIP6Rule

	lowPrioServiceRule, err := lookupQosRule(c.conn, ruleParams{
		table:        c.QosTable.Table,
		chain:        chain.Chain,
		ruleName:     LOWPRIOSERVICERULE,
		targetSet:    c.QosTable.ServiceSets.LowPrioServiceSet,
		keyExtractor: DstProtoPortExtractor(),
		ifaceSet:     c.QosTable.IfaceSet,
		mark:         int(priority.PRIORITYLOW),
	}, &c.opts)
	if err != nil {
		return err
	}
	chain.Rules.LowPrioServiceRule = lowPrioServiceRule

	return nil
}

func (c *NFT) initSets() error {
	highPrioIP4Set, err := lookupQosSet(c.conn, setParams{
		table:      c.QosTable.Table,
		setName:    HIGHPRIOIP4SETNAME,
		setType:    nftables.TypeIPAddr,
		isInterval: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.IPSets.HighPrioIP4Set = highPrioIP4Set

	highPrioIP6Set, err := lookupQosSet(c.conn, setParams{
		table:      c.QosTable.Table,
		setName:    HIGHPRIOIP6SETNAME,
		setType:    nftables.TypeIP6Addr,
		isInterval: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.IPSets.HighPrioIP6Set = highPrioIP6Set

	lowPrioIP4Set, err := lookupQosSet(c.conn, setParams{
		table:      c.QosTable.Table,
		setName:    LOWPRIOIP4SETNAME,
		setType:    nftables.TypeIPAddr,
		isInterval: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.IPSets.LowPrioIP4Set = lowPrioIP4Set

	lowPrioIP6Set, err := lookupQosSet(c.conn, setParams{
		table:      c.QosTable.Table,
		setName:    LOWPRIOIP6SETNAME,
		setType:    nftables.TypeIP6Addr,
		isInterval: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.IPSets.LowPrioIP6Set = lowPrioIP6Set

	highPrioServiceSet, err := lookupQosSet(c.conn, setParams{
		table:          c.QosTable.Table,
		setName:        HIGHPRIOSERVICESETNAME,
		setType:        nftables.MustConcatSetType(nftables.TypeInetProto, nftables.TypeInetService),
		isInterval:     false,
		concatentation: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.ServiceSets.HighPrioServiceSet = highPrioServiceSet

	lowPrioServiceSet, err := lookupQosSet(c.conn, setParams{
		table:          c.QosTable.Table,
		setName:        LOWPRIOSERVICESETNAME,
		setType:        nftables.MustConcatSetType(nftables.TypeInetProto, nftables.TypeInetService),
		isInterval:     false,
		concatentation: true,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.ServiceSets.LowPrioServiceSet = lowPrioServiceSet

	ifaceSet, err := lookupQosSet(c.conn, setParams{
		table:        c.QosTable.Table,
		setName:      IFACESETNAME,
		setType:      nftables.TypeIFName,
		isInterval:   false,
		keyEndianess: binaryutil.NativeEndian,
	}, &c.opts)
	if err != nil {
		return err
	}
	c.QosTable.IfaceSet = ifaceSet

	return nil
}

func lookupQosTable(conn *nftables.Conn, opts *NFTOpts) (*nftables.Table, error) {
	util.Debug(opts.Logger, "nft: lookup of qosm table")
	tables, err := conn.ListTables()
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		if table.Name == TABLENAME {
			util.Debug(opts.Logger, "nft: lookup successfull", "name", "qosmtable")
			return table, nil
		}
	}

	if opts.CreateIfNotExists {
		return createQosTable(conn, opts.Logger)
	}

	return nil, ErrTableNotFound
}

// createQosTable creates and adds a new qosm nftables table to the system.
// Returns the created table or an error if failed
func createQosTable(conn *nftables.Conn, logger *slog.Logger) (*nftables.Table, error) {
	util.Debug(logger, "nft: creating table", "name", "qosmtable")
	table := conn.AddTable(&nftables.Table{
		Name:   TABLENAME,
		Family: nftables.TableFamilyINet,
	})

	err := conn.Flush()
	if err != nil {
		return nil, err
	}

	return table, nil
}

// lookupQoSMChains searches for the specified chain within the specified nftables table.
// If found, it return the chain. If not found, it creates the chain
func lookupQosChain(conn *nftables.Conn, params chainParams, opts *NFTOpts) (QosChain, error) {
	util.Debug(opts.Logger, "nft: lookup of qosm chain")
	chains, err := conn.ListChains()
	if err != nil {
		return QosChain{}, err
	}

	for _, chain := range chains {
		if chain.Table.Name != params.table.Name {
			continue
		}
		if chain.Name == params.chainName {
			util.Debug(opts.Logger, "nft: chain lookup successfull", "name", params.chainName)
			return QosChain{
				Chain: chain,
			}, nil
		}
	}

	if opts.CreateIfNotExists {
		return createQosChain(conn, params, opts.Logger)
	}

	return QosChain{}, ErrChainNotFound
}

// createQosChain creates and adds a new chain to the specified nftables table.
// The chain is configured as the specified hook  with standard filter priority.
func createQosChain(conn *nftables.Conn, params chainParams, logger *slog.Logger) (QosChain, error) {
	util.Debug(logger, "nft: creating chain", "name", params.chainName)
	chainPolicy := params.chainPolicy

	chain := conn.AddChain(&nftables.Chain{
		Name:     params.chainName,
		Hooknum:  params.hook,
		Type:     nftables.ChainTypeFilter,
		Table:    params.table,
		Priority: nftables.ChainPriorityFilter,
		Policy:   &chainPolicy,
	})

	err := conn.Flush()
	if err != nil {
		return QosChain{}, err
	}

	return QosChain{
		Chain: chain,
	}, nil
}

func lookupQosRule(conn *nftables.Conn, params ruleParams, opts *NFTOpts) (*nftables.Rule, error) {
	util.Debug(opts.Logger, "nft: lookup of qosm rules", "name", params.ruleName, "chain", params.chain.Name)

	rules, err := conn.GetRules(params.table, params.chain)
	if err != nil {
		return nil, err
	}

	var qosRule *nftables.Rule

	for _, rule := range rules {
		if string(rule.UserData) == params.ruleName {
			util.Debug(opts.Logger, "nft: rule lookup successfull", "name", params.ruleName, "chain", params.chain.Name)
			qosRule = rule
		}
	}

	if qosRule == nil && opts.CreateIfNotExists {
		qosRule, err = createQosIPRule(conn, params, opts.Logger)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrRuleNotFound{Name: params.ruleName}
	}

	return qosRule, nil
}

func createQosIPRule(conn *nftables.Conn, params ruleParams, logger *slog.Logger) (*nftables.Rule, error) {
	util.Debug(logger, "nft: creating rule", "chain", params.chain.Name, "rule", params.ruleName, "mark", params.mark)
	byteMark := make([]byte, 4)
	binary.NativeEndian.PutUint32(byteMark, uint32(params.mark))

	exprs := make([]expr.Any, 0, 10) // The set of low level instructions for the nftables Virtual Machine.

	exprsToCheckForL3Proto := []expr.Any{
		// Load the network layer protocol of the packet into register 1
		&expr.Meta{
			Register: unix.NFT_REG_1,
			Key:      expr.MetaKeyNFPROTO,
		},
		// check if the network layer protocol in reg 1 is equal to the one required
		&expr.Cmp{
			Register: unix.NFT_REG_1,
			Data:     []byte{byte(params.l3proto)},
		},
	}
	if params.l3proto != 0 {
		// IF at all the layer 3 protocol is given, the following expressions are added to strictly match that protocol
		exprs = append(exprs, exprsToCheckForL3Proto...)
	}

	// these exprs given in params load the key of the given set eg ip daddr of the packet into register 1.
	exprs = append(exprs, params.keyExtractor...)

	exprsToLookUpKeyInTargetSet := []expr.Any{
		// The following expression checks if the payload in reg 1 loaded by params.keyExtractor is contained within the target set
		// for example if the keyExtractor is DstIPv4Extractor() the reg 1 will contain the ip dst addr and the expression
		// will check if that ip is contained in the given target set.
		&expr.Lookup{
			SourceRegister: unix.NFT_REG_1,
			SetName:        params.targetSet.Name,
			SetID:          params.targetSet.ID,
		},
	}
	exprs = append(exprs, exprsToLookUpKeyInTargetSet...)

	exprsToCheckForOutgoingIface := []expr.Any{
		// The following expressions check if the outgoing interface of the packet is contained within the qos_enabled_ifaces set
		// Load outgoing interface name into reg 1
		&expr.Meta{
			Register: unix.NFT_REG_1,
			Key:      expr.MetaKeyOIFNAME,
		},

		// Check if the outgoing interface's name is part of the interface set.
		&expr.Lookup{
			SourceRegister: unix.NFT_REG_1,
			SetName:        params.ifaceSet.Name,
			SetID:          params.ifaceSet.ID,
		},
	}
	exprs = append(exprs, exprsToCheckForOutgoingIface...)

	exprsToSetMarkAndReturn := []expr.Any{
		// The following expressions set the appropriate mark to the matched packets.
		// Load the mark into register 1
		&expr.Immediate{
			Register: unix.NFT_REG_1,
			Data:     byteMark,
		},

		// Set the mark field in the metadata with what is in register 1.
		&expr.Meta{
			Key:            expr.MetaKeyMARK,
			SourceRegister: true, // indicates that we are reading  from the register not writing to it.
			Register:       unix.NFT_REG_1,
		},

		// Add a counter to the rule for the matched packets.
		&expr.Counter{},

		// Stop proceesing rules for this chain
		&expr.Verdict{
			Kind: expr.VerdictReturn,
		},
	}
	exprs = append(exprs, exprsToSetMarkAndReturn...)

	rule := conn.AddRule(&nftables.Rule{
		Table:    params.table,
		Chain:    params.chain,
		UserData: []byte(params.ruleName),
		Exprs:    exprs,
	})

	err := conn.Flush()
	if err != nil {
		return nil, err
	}

	return rule, nil
}

func lookupQosSet(conn *nftables.Conn, params setParams, opts *NFTOpts) (*nftables.Set, error) {
	util.Debug(opts.Logger, "nft: lookup of qosm set ", params.setName)

	nftSet, err := conn.GetSetByName(params.table, params.setName)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) && opts.CreateIfNotExists {
			nftSet, err = createQosIPSet(conn, params, opts.Logger)
			if err != nil {
				return nil, err
			}
		} else if errors.Is(err, os.ErrNotExist) {
			return nil, ErrSetNotFound{Name: params.setName}
		} else {
			return nil, err
		}
	} else {
		util.Debug(opts.Logger, "nft: set found", "name", HIGHPRIOIP4SETNAME)
	}

	return nftSet, nil
}

// createQosIPSet creates and adds a new IP address set to the specified nftables table.
// The set is configured to store IPv4 addresses and is initialized as empty.
// Returns the created set or an error if flushing fails.
func createQosIPSet(conn *nftables.Conn, params setParams, logger *slog.Logger) (*nftables.Set, error) {
	util.Debug(logger, "nft: creating set", "name", params.setName)
	set := &nftables.Set{
		Table:         params.table,
		Name:          params.setName,
		KeyType:       params.setType,
		Concatenation: params.concatentation,
		Interval:      params.isInterval,
		KeyByteOrder:  params.keyEndianess,
	}
	ipSetElements := []nftables.SetElement{}

	err := conn.AddSet(set, ipSetElements)
	if err != nil {
		return nil, err
	}

	err = conn.Flush()
	if err != nil {
		return nil, err
	}

	return set, nil
}
