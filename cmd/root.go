package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

func NewRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "crypto-tracker",
		Short: "Import Crypto.com transactions into google sheets",
	}
	cobra.OnInitialize(initConfig)

	command.AddCommand(NewLoginCommand())
	command.AddCommand(NewImportCommand())

	command.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.crypto-tracker.yaml)")
	command.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	return command
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigName(".crypto-tracker")
	}

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
