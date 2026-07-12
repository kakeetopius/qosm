package htb

import (
	"errors"
	"log/slog"

	"github.com/florianl/go-tc"
)

func InitHTBOnIface(tcnl *tc.Tc, ifIndex int, rate uint32, percentages ClassPercentages, logger *slog.Logger) error {
	_, err := findRootQdisc(tcnl, ifIndex)
	if err == nil {
		return ErrQdiscExists
	} else if !errors.Is(err, ErrQdiscNotFound) {
		return err
	}

	_, err = CreateQdisc(tcnl, ifIndex, rate, percentages, logger)
	if err != nil {
		return err
	}

	return nil
}

func FlushQdiscFromIface(tcnl *tc.Tc, ifIndex int) error {
	root, err := findRootQdisc(tcnl, ifIndex)
	if err != nil {
		return err
	}
	return deleteQdisc(tcnl, root)
}

func HasHTBQdisc(tcnl *tc.Tc, ifIndex int) (bool, error) {
	_, err := findRootQdisc(tcnl, ifIndex)
	if err == nil {
		return true, nil
	} else if errors.Is(err, ErrQdiscNotFound) {
		return false, nil
	}
	return false, err
}

func GetClassStats(class *tc.Object) (stats HTBClassStats) {
	if class == nil {
		return
	}

	if class.Stats != nil {
		stats.Bytes = class.Stats.Bytes
		stats.Packets = class.Stats.Packets
		stats.Drops = class.Stats.Drops
		stats.Overlimits = class.Stats.Overlimits
		stats.Bps = class.Stats.Bps
		stats.Pps = class.Stats.Pps
		stats.Qlen = class.Stats.Qlen
		stats.Backlog = class.Stats.Backlog
	}

	if class.XStats != nil && class.XStats.Htb != nil {
		stats.Lends = class.XStats.Htb.Lends
		stats.Borrows = class.XStats.Htb.Borrows
		stats.Giants = class.XStats.Htb.Giants
		stats.Tokens = class.XStats.Htb.Tokens
		stats.CTokens = class.XStats.Htb.CTokens
	}

	return
}

func (stats *HTBClassStats) Aggregate(stats2 HTBClassStats) *HTBClassStats {
	stats.Bytes += stats2.Bytes
	stats.Packets += stats2.Packets

	return stats
}
