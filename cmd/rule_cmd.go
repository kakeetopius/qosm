package cmd

import (
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/spf13/cobra"
)

func RuleCmd() *cobra.Command {
	ruleCmd := cobra.Command{
		Use:     "rules",
		Short:   "Manage QoS rules.",
		Aliases: []string{"r"},
	}

	ruleCmd.AddCommand(
		HostRuleCmd(),
		ServiceRuleCmd(),
		RuleListCmd(),
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
		Use:   "service",
		Short: "Manage service rules i.e rules that match based on port and protocol.",
		Example: `Services can be given in the form protocol/port e.g 
  tcp/80 udp/53	tcp/22 tcp/443 udp/900`,
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
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
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

func RuleListCmd() *cobra.Command {
	ruleListCmd := cobra.Command{
		Use:     "list",
		Short:   "List all QoS rules",
		Aliases: []string{"l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			defer qosManager.Close()

			highPrio, err := qosManager.GetHighPriorityRules()
			if err != nil {
				return err
			}
			lowPrio, err := qosManager.GetLowPriorityRules()
			if err != nil {
				return err
			}

			return printRules(highPrio, lowPrio)
		},
	}

	return &ruleListCmd
}
