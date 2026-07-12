package cmd

import (
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/spf13/cobra"
)

func IfaceCmd() *cobra.Command {
	ifaceCmd := cobra.Command{
		Use:     "iface",
		Short:   "Manage traffic control settings for an interface.",
		Aliases: []string{"i"},
	}

	ifaceCmd.AddCommand(
		IfaceEnableCmd(),
		IfaceDisableCmd(),
		IfaceListCmd(),
		IfaceSetCmd(),
		IfaceStats(),
	)
	return &ifaceCmd
}

func IfaceEnableCmd() *cobra.Command {
	var (
		rate        = new(uint32)
		percentages []string
	)
	defaultPercentages := []string{"50", "40", "10"}

	ifaceEnableCmd := cobra.Command{
		Use:     "enable interface_names...",
		Short:   "Enable the htb qdisc on an interface(s)",
		Aliases: []string{"e"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: true,
			})
			if err != nil {
				return err
			}
			defer qosManager.Close()

			classPercentages := &htb.ClassPercentages{}
			if cmd.Flags().Changed("percentages") {
				if len(percentages) != 3 {
					return fmt.Errorf("please provide three percentages in the form high,default,low")
				}
				p, perr := htb.ClassPercentagesFromStrings(percentages[0], percentages[1], percentages[2])
				if perr != nil {
					return perr
				}
				classPercentages = &p
				perr = classPercentages.Verify()
				if perr != nil {
					return perr
				}
			} else {
				classPercentages = nil
			}

			if !cmd.Flags().Changed("rate") {
				rate = nil
			}

			for _, iface := range args {
				err = qosManager.EnableTcOnInterface(iface, rate, classPercentages)
				if err != nil {
					return fmt.Errorf(" Interface %v -> %w", iface, err)
				}
				fmt.Printf("Successfully enabled HTB qdisc on interface: %v\n", iface)
			}

			return nil
		},
	}

	ifaceEnableCmd.Flags().Uint32VarP(rate, "rate", "r", qos.DEFAULTRATE, "The rate in Mbps to divide among the different priority classes.")
	ifaceEnableCmd.Flags().StringSliceVarP(&percentages, "percentages", "p", defaultPercentages, "The percentages for each priority class in form high,default,low")

	return &ifaceEnableCmd
}

func IfaceSetCmd() *cobra.Command {
	var (
		rate        uint32
		percentages []string
	)

	ifaceEnableCmd := cobra.Command{
		Use:   "set interface_name",
		Short: "Modify the interface QoS paramaters",
		Long: `Modify the interface QoS paramaters. 

Note that altering some paramaters like the rate or the percentage rates for each class 
requires tearing down the qdisc from the interface and creating a new one which in turn clears all accumulated counters.`,
		Aliases: []string{"s"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: true,
			})
			if err != nil {
				return err
			}
			defer qosManager.Close()

			if cmd.Flags().Changed("rate") {
				err = qosManager.ChangeInterfaceRate(args[0], rate)
				if err != nil {
					return err
				}
				fmt.Printf("Successfully changed the total rate for %v to %v\n", args[0], rate)
			}

			if cmd.Flags().Changed("percentages") {
				if len(percentages) != 3 {
					return fmt.Errorf("please provide three percentages in the form high,default,low")
				}
				classPercentages, err := htb.ClassPercentagesFromStrings(percentages[0], percentages[1], percentages[2])
				if err != nil {
					return err
				}
				err = classPercentages.Verify()
				if err != nil {
					return err
				}
				err = qosManager.ChangeClassPercentages(args[0], classPercentages)
				if err != nil {
					return err
				}
				fmt.Printf("Successfully changed the class percentages rates for %v to %v\n", args[0], classPercentages.String())
			}

			return nil
		},
	}

	ifaceEnableCmd.Flags().Uint32VarP(&rate, "rate", "r", 0, "Set the rate in Mbps to divide among the different priority classes.")
	ifaceEnableCmd.Flags().StringSliceVarP(&percentages, "percentages", "p", nil, "Set the percentages for each priority class in form high,default,low")

	return &ifaceEnableCmd
}

func IfaceDisableCmd() *cobra.Command {
	ifaceDisableCmd := cobra.Command{
		Use:     "disable interface_names...",
		Short:   "Disable the htb qdisc from an interface(s)",
		Aliases: []string{"d"},
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: true,
			})
			if err != nil {
				return err
			}
			defer qosManager.Close()

			for _, iface := range args {
				err = qosManager.DisableTcOnInterface(iface)
				if err != nil {
					return fmt.Errorf(" Interface %v -> %w", iface, err)
				}
				fmt.Printf("Successfully disabled the HTB qdisc on interface: %v\n", iface)
			}

			return nil
		},
	}

	return &ifaceDisableCmd
}

func IfaceListCmd() *cobra.Command {
	ifacelistCmd := cobra.Command{
		Use:     "list",
		Short:   "List htb enabled interfaces.",
		Aliases: []string{"l"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			enabledIfaces := qosManager.EnabledInterfaces()
			if len(enabledIfaces) == 0 {
				fmt.Println("No htb enabled interfaces.")
				return nil
			}

			HeadingPrinter.Println("Enabled Interfaces")
			return printIfaces(enabledIfaces)
		},
	}

	return &ifacelistCmd
}

func IfaceStats() *cobra.Command {
	ifacelistCmd := cobra.Command{
		Use:     "stats",
		Short:   "Get stats for an interface",
		Aliases: []string{"s"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			qosManager, err := getQosManager(nft.NFTOpts{
				CreateTableIfNotExists: false,
			})
			if err != nil && !errors.Is(err, nft.ErrTableNotFound) {
				return err
			}
			ifaceStats, err := qosManager.GetIfaceStats(args[0])
			if err != nil {
				return err
			}
			return printIfaceStats(&ifaceStats)
		},
	}

	return &ifacelistCmd
}
