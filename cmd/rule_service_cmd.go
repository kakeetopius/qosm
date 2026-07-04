package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/kakeetopius/qosm/internal/service"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func ServiceRuleAddCmd() *cobra.Command {
	var priority string
	ruleAddCmd := cobra.Command{
		Use:   "add service...",
		Short: "Add a QoS rule(s) that matches a service i.e protocol and port.",
		Example: `  qosm rule service add tcp/443 --priority high
  qosm rule service add tcp/80 udp/53 tcp/22 --priority high`,
		Aliases: []string{"a"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toAdd := make([]service.Service, 0, len(args))
			for _, serviceSpec := range args {
				serv, err := service.ServiceFromString(serviceSpec)
				if err != nil {
					return err
				}
				toAdd = append(toAdd, serv)
			}

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

			for _, serv := range toAdd {
				_, err := qosManager.AddServiceRule(serv, priority)
				if err != nil {
					return err
				}
				fmt.Printf("Service rule for %v added successfully\n", serv)
			}

			return nil
		},
	}

	ruleAddCmd.Flags().StringVarP(&priority, "priority", "p", "", "Priority for the given services.")
	ruleAddCmd.MarkFlagRequired("priority")

	return &ruleAddCmd
}

func ServiceRuleDeleteCmd() *cobra.Command {
	ruleDeleteCmd := cobra.Command{
		Use:   "delete service...",
		Short: "Delete a QoS rule(s) that matches a service",
		Example: `  qosm rule service delete tcp/443
  qosm rule service delete tcp/80 udp/53 tcp/22`,
		Aliases: []string{"d"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toDelete := make([]service.Service, 0, len(args))
			for _, serviceSpec := range args {
				serv, err := service.ServiceFromString(serviceSpec)
				if err != nil {
					return err
				}
				toDelete = append(toDelete, serv)
			}
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

			for _, serv := range toDelete {
				err := qosManager.DeleteServiceRule(serv)
				if err != nil {
					return err
				}
				fmt.Printf("Service rule for %v deleted successfully\n", serv)
			}

			return nil
		},
	}

	return &ruleDeleteCmd
}

func ServiceRuleListCmd() *cobra.Command {
	ruleListCmd := cobra.Command{
		Use:     "list",
		Short:   "List all QoS rules that match services",
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

			highPrio, err := qosManger.GetHighPriorityServices()
			if err != nil {
				return err
			}
			lowPrio, err := qosManger.GetLowPriorityServices()
			if err != nil {
				return err
			}

			highPrioTable := pterm.DefaultTable
			highPrioTableData := pterm.TableData{
				{"ID", "Service", "Created At"},
			}
			for _, rule := range highPrio {
				highPrioTableData = append(highPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.String(), rule.CreatedAt.String()})
			}

			lowPrioTable := pterm.DefaultTable
			lowPrioTableData := pterm.TableData{
				{"ID", "Service", "Created At"},
			}
			for _, rule := range lowPrio {
				lowPrioTableData = append(lowPrioTableData, []string{fmt.Sprintf("%v", rule.ID), rule.String(), rule.CreatedAt.String()})
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
