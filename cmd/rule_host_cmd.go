package cmd

import (
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/core/nft"
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
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: true,
			})
			if err != nil {
				return err
			}
			defer qosManager.Close()

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
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			defer qosManager.Close()

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
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			defer qosManager.Close()

			highPrio, err := qosManager.GetHighPriorityHostRules()
			if err != nil {
				return err
			}
			lowPrio, err := qosManager.GetLowPriorityHostRules()
			if err != nil {
				return err
			}

			return printRules(highPrio, lowPrio)
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
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			defer qosManager.Close()

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
