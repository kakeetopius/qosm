package cmd

import (
	"fmt"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/spf13/cobra"
)

func RuleCmd() *cobra.Command {
	ruleCmd := cobra.Command{
		Use:     "rule",
		Short:   "Manage QoS rules.",
		Aliases: []string{"r"},
	}

	ruleCmd.AddCommand(
		HostRuleCmd(),
		ServiceRuleCmd(),
		RuleFlushCmd(),
	)
	return &ruleCmd
}

func HostRuleCmd() *cobra.Command {
	hostRuleCmd := cobra.Command{
		Use:     "host",
		Short:   "Manage host rules i.e rules that match based on ip address or domain name",
		Aliases: []string{"h"},
	}

	hostRuleCmd.AddCommand(
		HostRuleAddCmd(),
		HostRuleDeleteCmd(),
		HostRuleListCmd(),
		HostRuleRefreshCmd(),
	)
	return &hostRuleCmd
}

func ServiceRuleCmd() *cobra.Command {
	serviceRuleCmd := cobra.Command{
		Use:     "service",
		Short:   "Manage service rules i.e rules that match based on port and protocol.",
		Aliases: []string{"serv", "s"},
	}

	serviceRuleCmd.AddCommand(
		ServiceRuleAddCmd(),
		ServiceRuleDeleteCmd(),
		ServiceRuleListCmd(),
	)

	return &serviceRuleCmd
}

func RuleFlushCmd() *cobra.Command {
	ruleFlushCmd := cobra.Command{
		Use:     "flush",
		Short:   "Flush all QoS rules.",
		Aliases: []string{"f"},
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

			err = qosManager.DeleteAllRules()
			if err != nil {
				return err
			}
			fmt.Println("Rules flushed successfully.")
			return nil
		},
	}

	return &ruleFlushCmd
}
