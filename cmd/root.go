// Package cmd is used for command line parsing and configuration setup
package cmd

import (
	"cmp"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path"
	"path/filepath"

	goversion "github.com/caarlos0/go-version"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/qos"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	debug      bool
	deamonMode bool

	appConfig *viper.Viper
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:          "qosm",
	Short:        "A quality of service manager.",
	SilenceUsage: true,
	Version:      buildVersion().GitVersion,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	cobra.EnableCommandSorting = false
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	appConfig = viper.New()

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "The path to the config file (default is $HOME/config/qosm/qosm.toml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Run in debug mode")

	rootCmd.PersistentFlags().String("db-path", "", "The path to the database file. ($HOME/config/qosm/qos.db)")
	appConfig.BindPFlag("db.path", rootCmd.PersistentFlags().Lookup("db-path"))

	rootCmd.PersistentFlags().BoolVarP(&deamonMode, "daemon-mode", "d", false, "Run in daemon mode. In this mode priviliged operations like enabling tc on an interface are sent to the qos daemon")

	rootCmd.PersistentFlags().String("sock", "", "The path to the qos daemon socket if running in daemon mode (default is /run/qosd/qosd.sock)")
	appConfig.BindPFlag("daemon.sock", rootCmd.PersistentFlags().Lookup("sock"))
	appConfig.SetDefault("daemon.sock", "/run/qosd/qosd.sock")

	rootCmd.AddCommand(
		versionCmd(),
		RuleCmd(),
		IfaceCmd(),
		StatsCmd(),
		RestoreCmd(),
		WebCmd(),
		DaemonCmd(),
	)
}

func initConfig() error {
	if cfgFile != "" {
		// Use config file from the flag.
		appConfig.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		configDir, err := configDir()
		if err != nil {
			return err
		}

		// Search config in config directory with name "qosm"
		appConfig.AddConfigPath(path.Join(configDir, "qosm"))

		appConfig.SetConfigName("qosm")
	}

	defer printConfigs()

	// set default database path
	dbPath, err := getDefaultDBPath()
	if err != nil {
		return err
	}
	appConfig.SetDefault("db.path", dbPath)

	// If a config file is found, read it in.
	err = appConfig.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// ignore error file not found
			return nil
		}
		return fmt.Errorf("error reading config file %v: %w", appConfig.ConfigFileUsed(), err)
	}

	return nil
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show detailed version information",
		Aliases: []string{"v"},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(buildVersion().String())
		},
	}
}

// configDir returns the user's configuration directory.
// When running as root, it resolves the original sudo user's home directory
// and returns its .config path. Otherwise, it uses the current user's config
// directory.
func configDir() (string, error) {
	home := ""
	if os.Geteuid() == 0 {
		// running as root
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser == "" { // normal root user -> not sudo
			return os.UserConfigDir()
		}
		u, err := user.Lookup(sudoUser)
		if err != nil {
			return "", err
		}
		home = u.HomeDir
		return path.Join(home, ".config"), nil
	} else {
		return os.UserConfigDir()
	}
}

func getDefaultDBPath() (string, error) {
	configDir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "qosm", "qos.db"), nil
}

func buildVersion() goversion.Info {
	return goversion.GetVersionInfo(
		goversion.WithAppDetails("qosm", "A quality of service manager", ""),
	)
}

func printConfigs() {
	if !debug {
		return
	}
	fmt.Fprintln(os.Stderr, "Using db file:", appConfig.GetString("db.path"))
	fmt.Fprintln(os.Stderr, "Using config file:", cmp.Or(appConfig.ConfigFileUsed(), "none"))
	fmt.Fprintln(os.Stderr, "Using daemon socket:", appConfig.GetString("daemon.sock"))
	fmt.Fprintln(os.Stderr, "Running in daemon mode: ", deamonMode)

	fmt.Fprintln(os.Stderr)
}

func getQosManager(nftOpts nft.NFTOpts) (*qos.QoSManager, error) {
	dbConn, err := db.NewConn(appConfig.GetString("db.path"))
	if err != nil {
		return nil, err
	}

	loggerOpts := slog.HandlerOptions{}
	if debug {
		loggerOpts.Level = slog.LevelDebug
	}
	qosManager, err := qos.NewManager(qos.Options{
		DB:         dbConn,
		DaemonMode: deamonMode,
		DaemonSock: appConfig.GetString("daemon.sock"),
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &loggerOpts)),
	})
	if err != nil {
		return nil, err
	}

	err = qosManager.InitQoSClassifier(nftOpts)
	if err != nil {
		return nil, err
	}

	return qosManager, nil
}
