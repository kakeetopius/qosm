package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func HostRuleAddCmd() *cobra.Command {
	var priority string
	var ruleType string
	ruleAddCmd := cobra.Command{
		Use:     "add ip/domain...",
		Short:   "Add a rule(s) that matches a host i.e. based on ip address or domain name",
		Aliases: []string{"a"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbConn, err := db.NewConn(appConfig.GetString("db.path"))
			if err != nil {
				return err
			}
			defer dbConn.Close()

			qosManager, err := qos.NewManager(dbConn)
			if err != nil {
				return err
			}
			defer qosManager.Close()

			if debug {
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				}))
				qosManager.WithLogger(logger)
			}

			err = qosManager.InitQoSClassifier(true)
			if err != nil {
				return err
			}
			for _, rule := range args {
				switch ruleType {
				case "ip":
					_, err = qosManager.AddIPRule(rule, priority)
				case "domain":
					_, err = qosManager.AddDomainRule(rule, priority)
				default:
					err = fmt.Errorf("unknown rule type: %s", ruleType)
				}
				if err != nil {
					return err
				}

				fmt.Printf("Rule for %v added successfully\n", rule)
			}
			return nil
		},
	}

	ruleAddCmd.Flags().StringVarP(&priority, "priority", "p", "", "Priority for the given targets.")
	ruleAddCmd.MarkFlagRequired("priority")
	ruleAddCmd.Flags().StringVarP(&ruleType, "type", "t", "ip", "The type of the target i.e. ip or domain")

	return &ruleAddCmd
}

func HostRuleDeleteCmd() *cobra.Command {
	var ruleType string
	ruleDeleteCmd := cobra.Command{
		Use:     "delete ip/domain...",
		Short:   "Delete a QoS rule(s) that matches a host(s)",
		Aliases: []string{"d"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dbConn, err := db.NewConn(appConfig.GetString("db.path"))
			if err != nil {
				return err
			}
			defer dbConn.Close()

			qosManager, err := qos.NewManager(dbConn)
			if err != nil {
				return err
			}
			defer qosManager.Close()

			if debug {
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				}))
				qosManager.WithLogger(logger)
			}

			err = qosManager.InitQoSClassifier(false)
			if err != nil {
				if errors.Is(err, nft.ErrTableNotFound) {
					return fmt.Errorf(" No tc rules added yet by qosm ")
				}
				return err
			}

			for _, rule := range args {
				switch ruleType {
				case "domain":
					err = qosManager.DeleteDomainRuleByName(rule)
				case "ip":
					err = qosManager.DeleteIPRuleByName(rule)
				default:
					err = fmt.Errorf("unknown rule type: %s", ruleType)
				}

				if err != nil {
					return err
				}

				fmt.Printf("Rule for %v deleted successfully\n", rule)
			}

			return nil
		},
	}

	ruleDeleteCmd.Flags().StringVarP(&ruleType, "type", "t", "ip", "The type of the target i.e. ip or domain")

	return &ruleDeleteCmd
}

func HostRuleListCmd() *cobra.Command {
	ruleListCmd := cobra.Command{
		Use:     "list",
		Short:   "List all QoS rules that match hosts",
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbConn, err := db.NewConn(appConfig.GetString("db.path"))
			if err != nil {
				return err
			}
			defer dbConn.Close()

			qosManger, err := qos.NewManager(dbConn)
			if err != nil {
				return err
			}
			defer qosManger.Close()

			highPrio, err := qosManger.GetHighPriorityHostRules()
			if err != nil {
				return err
			}
			lowPrio, err := qosManger.GetLowPriorityHostRules()
			if err != nil {
				return err
			}

			highPrioTable := pterm.DefaultTable
			highPrioTableData := pterm.TableData{
				{"ID", "Type", "Target", "Created At"},
			}
			for _, rule := range highPrio {
				highPrioTableData = append(highPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.Type, rule.Target, rule.CreatedAt.String()})
			}

			lowPrioTable := pterm.DefaultTable
			lowPrioTableData := pterm.TableData{
				{"ID", "Type", "Target", "Created At"},
			}
			for _, rule := range lowPrio {
				lowPrioTableData = append(lowPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.Type, rule.Target, rule.CreatedAt.String()})
			}

			if len(highPrio) > 0 {
				fmt.Println("High Priority Rules")
				highPrioTable.WithHasHeader(true).WithHeaderRowSeparator("-").WithBoxed(true).WithData(highPrioTableData).Render()
			}
			if len(lowPrio) > 0 {
				fmt.Println("Low Priority Rules")
				lowPrioTable.WithHasHeader(true).WithHeaderRowSeparator("-").WithBoxed(true).WithData(lowPrioTableData).Render()
			}

			return nil
		},
	}

	return &ruleListCmd
}

func HostRuleRefreshCmd() *cobra.Command {
	ruleRefresh := cobra.Command{
		Use:     "refresh-dns",
		Short:   "Refresh dns mappings for stored domain rules.",
		Aliases: []string{"r"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Refreshing Domains..................")
			dbCon, err := db.NewConn(appConfig.GetString("db.path"))
			if err != nil {
				return err
			}
			defer dbCon.Close()

			qosManager, err := qos.NewManager(dbCon)
			if err != nil {
				return err
			}
			defer qosManager.Close()

			if debug {
				logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
					Level: slog.LevelDebug,
				}))
				qosManager.WithLogger(logger)
			}

			err = qosManager.InitQoSClassifier(false)
			if err != nil {
				if errors.Is(err, nft.ErrTableNotFound) {
					return fmt.Errorf(" No tc rules added yet by qosm ")
				}
				return err
			}

			err = qosManager.RefreshAllDomains()
			if err != nil {
				return err
			}
			fmt.Println("Refresh Successfully Completed")
			return nil
		},
	}

	return &ruleRefresh
}
