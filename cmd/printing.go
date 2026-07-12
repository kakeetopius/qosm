package cmd

import (
	"fmt"

	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/pterm/pterm"
)

var HeadingPrinter = pterm.DefaultSection

func init() {
	HeadingPrinter.BottomPadding = 0
}

func printRules(highPrio []qos.Rule, lowPrio []qos.Rule) error {
	highPrioTable := pterm.DefaultTable
	highPrioTableData := pterm.TableData{
		{"ID", "Type", "Target", "Created At"},
	}
	for _, rule := range highPrio {
		highPrioTableData = append(highPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.Type, rule.Target, rule.CreatedAt.Local().String()})
	}

	lowPrioTable := pterm.DefaultTable
	lowPrioTableData := pterm.TableData{
		{"ID", "Type", "Target", "Created At"},
	}
	for _, rule := range lowPrio {
		lowPrioTableData = append(lowPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.Type, rule.Target, rule.CreatedAt.Local().String()})
	}

	if len(highPrio) > 0 {
		HeadingPrinter.Println("High Priority Rules")
		err := highPrioTable.
			WithHasHeader(true).
			WithBoxed(true).
			WithData(highPrioTableData).
			Render()
		if err != nil {
			return err
		}
	}
	if len(lowPrio) > 0 {
		HeadingPrinter.Println("Low Priority Rules")
		err := lowPrioTable.
			WithHasHeader(true).
			WithBoxed(true).
			WithData(lowPrioTableData).
			Render()
		if err != nil {
			return err
		}

	}

	return nil
}

func printStats(stats *qos.QoSStats) error {
	if stats == nil {
		return nil
	}
	HeadingPrinter.Println("QoS Statistics")

	summary := pterm.TableData{
		{"Metric", "Value"},
		{"Managed Interfaces", fmt.Sprintf("%d", stats.ManagedIfaces)},
		{"Active Interfaces", fmt.Sprintf("%d", stats.ActiveIfaces)},
		{"Total Bytes", fmt.Sprintf("%d", stats.TotalBytes)},
		{"Total Packets", fmt.Sprintf("%d", stats.TotalPackets)},
		{"Total Drops", fmt.Sprintf("%d", stats.TotalDrops)},
		{"Total Overlimits", fmt.Sprintf("%d", stats.TotalOverlimits)},
	}

	err := pterm.DefaultTable.
		WithHasHeader().
		WithBoxed(true).
		WithData(summary).
		Render()
	if err != nil {
		return err
	}

	for _, iface := range stats.IfaceStats {
		if err := printIfaceStats(&iface); err != nil {
			continue
		}
	}

	return nil
}

func printIfaces(ifaces []qos.Interface) error {
	data := pterm.TableData{
		{
			"Interface",
			"Ifindex",
			"Rate (Mbps)",
			"High %",
			"Default %",
			"Low %",
		},
	}

	for _, iface := range ifaces {
		data = append(data, []string{
			iface.Name,
			fmt.Sprintf("%d", iface.Index),
			fmt.Sprintf("%d", iface.ShapingRate),
			fmt.Sprintf("%.1f", iface.Percentages.HighPrioClass),
			fmt.Sprintf("%.1f", iface.Percentages.DefaultClass),
			fmt.Sprintf("%.1f", iface.Percentages.LowPrioClass),
		})
	}

	return pterm.DefaultTable.
		WithHasHeader().
		WithData(data).
		WithBoxed(true).
		Render()
}

func printIfaceStats(stats *qos.IfaceStats) error {
	if stats == nil {
		return nil
	}
	HeadingPrinter.Println("Interface: " + stats.Name)

	data := pterm.TableData{
		{"Metric", "Total", "High", "Default", "Low"},
		{
			"Bytes",
			fmt.Sprintf("%d", stats.Total.Bytes),
			fmt.Sprintf("%d", stats.HighPrioClass.Bytes),
			fmt.Sprintf("%d", stats.DefaultClass.Bytes),
			fmt.Sprintf("%d", stats.LowPrioClass.Bytes),
		},
		{
			"Packets",
			fmt.Sprintf("%d", stats.Total.Packets),
			fmt.Sprintf("%d", stats.HighPrioClass.Packets),
			fmt.Sprintf("%d", stats.DefaultClass.Packets),
			fmt.Sprintf("%d", stats.LowPrioClass.Packets),
		},
		{
			"Drops",
			fmt.Sprintf("%d", stats.Total.Drops),
			fmt.Sprintf("%d", stats.HighPrioClass.Drops),
			fmt.Sprintf("%d", stats.DefaultClass.Drops),
			fmt.Sprintf("%d", stats.LowPrioClass.Drops),
		},
		{
			"Overlimits",
			fmt.Sprintf("%d", stats.Total.Overlimits),
			fmt.Sprintf("%d", stats.HighPrioClass.Overlimits),
			fmt.Sprintf("%d", stats.DefaultClass.Overlimits),
			fmt.Sprintf("%d", stats.LowPrioClass.Overlimits),
		},
		{
			"Backlog (bytes)",
			fmt.Sprintf("%d", stats.Total.Backlog),
			fmt.Sprintf("%d", stats.HighPrioClass.Backlog),
			fmt.Sprintf("%d", stats.DefaultClass.Backlog),
			fmt.Sprintf("%d", stats.LowPrioClass.Backlog),
		},
		{
			"Queue Length",
			fmt.Sprintf("%d", stats.Total.Qlen),
			fmt.Sprintf("%d", stats.HighPrioClass.Qlen),
			fmt.Sprintf("%d", stats.DefaultClass.Qlen),
			fmt.Sprintf("%d", stats.LowPrioClass.Qlen),
		},
		{
			"Lends",
			fmt.Sprintf("%d", stats.Total.Lends),
			fmt.Sprintf("%d", stats.HighPrioClass.Lends),
			fmt.Sprintf("%d", stats.DefaultClass.Lends),
			fmt.Sprintf("%d", stats.LowPrioClass.Lends),
		},
		{
			"Borrows",
			fmt.Sprintf("%d", stats.Total.Borrows),
			fmt.Sprintf("%d", stats.HighPrioClass.Borrows),
			fmt.Sprintf("%d", stats.DefaultClass.Borrows),
			fmt.Sprintf("%d", stats.LowPrioClass.Borrows),
		},
		{
			"Giants",
			fmt.Sprintf("%d", stats.Total.Giants),
			fmt.Sprintf("%d", stats.HighPrioClass.Giants),
			fmt.Sprintf("%d", stats.DefaultClass.Giants),
			fmt.Sprintf("%d", stats.LowPrioClass.Giants),
		},
		{
			"Tokens",
			fmt.Sprintf("%d", stats.Total.Tokens),
			fmt.Sprintf("%d", stats.HighPrioClass.Tokens),
			fmt.Sprintf("%d", stats.DefaultClass.Tokens),
			fmt.Sprintf("%d", stats.LowPrioClass.Tokens),
		},
		{
			"CTokens",
			fmt.Sprintf("%d", stats.Total.CTokens),
			fmt.Sprintf("%d", stats.HighPrioClass.CTokens),
			fmt.Sprintf("%d", stats.DefaultClass.CTokens),
			fmt.Sprintf("%d", stats.LowPrioClass.CTokens),
		},
	}

	return pterm.DefaultTable.
		WithHasHeader().
		WithBoxed(true).
		WithData(data).
		Render()
}
