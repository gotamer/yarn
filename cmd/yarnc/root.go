package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/client"
	"git.mills.io/yarnsocial/yarn/types/lextwt"
	"git.mills.io/yarnsocial/yarn/types/retwt"
)

const (
	DefaultConfigFilename = ".yarnc.yml"
	DefaultEnvPrefix      = "YARNC"
)

var (
	ConfigFile        string
	DefaultConfigFile string
)

func init() {
	homeDir, err := homedir.Dir()
	if err != nil {
		log.WithError(err).Fatal("error finding user home directory")
	}

	DefaultConfigFile = filepath.Join(homeDir, DefaultConfigFilename)
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:     "yarnc",
	Version: yarn.FullVersion(),
	Short:   "Command-line client for yarnd",
	Long: `This is the command-line client for Yarn.social pods running
yarnd. This tool allows a user to interact with a pod to view their timeline,
following feeds, make posts and managing their account.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set logging level
		if viper.GetBool("debug") {
			log.SetLevel(log.DebugLevel)
		} else {
			log.SetLevel(log.InfoLevel)
		}
	},
}

// Execute adds all child commands to the root command
// and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.WithError(err).Error("error executing command")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVarP(
		&ConfigFile, "config", "c", DefaultConfigFile,
		"set a custom config file",
	)

	RootCmd.PersistentFlags().BoolP(
		"debug", "d", false,
		"Enable debug logging",
	)

	parser := RootCmd.PersistentFlags().StringP(
		"parser", "p", "lextwt",
		"Set active parse engine [lextwt, retwt]",
	)

	RootCmd.PersistentFlags().StringP(
		"uri", "u", client.DefaultURI,
		"yarnd API endpoint URI to connect to",
	)

	RootCmd.PersistentFlags().StringP(
		"token", "t", fmt.Sprintf("$%s_TOKEN", DefaultEnvPrefix),
		"yarnd API token to use to authenticate to endpoints",
	)

	viper.BindPFlag("uri", RootCmd.PersistentFlags().Lookup("uri"))
	viper.SetDefault("uri", client.DefaultURI)

	viper.BindPFlag("token", RootCmd.PersistentFlags().Lookup("token"))
	viper.SetDefault("token", os.Getenv(fmt.Sprintf("%_TOKEN", DefaultEnvPrefix)))

	viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	viper.SetDefault("debug", false)

	// I have no idea how to work with cobra :)
	// put this someplace to run on startup.
	switch *parser {
	case "lextwt":
		lextwt.DefaultTwtManager()
	case "retwt":
		retwt.DefaultTwtManager()
	default:
		log.Fatalf("unknown parse engine: %s", *parser)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetConfigFile(ConfigFile)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		log.WithError(err).Warnf("error loading config file: %s", viper.ConfigFileUsed())
	} else {
		log.Debugf("Using config file: %s", viper.ConfigFileUsed())
	}

	// from the environment
	viper.SetEnvPrefix(DefaultEnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match
}
