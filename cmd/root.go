// Copyright © 2017 Meyer Zinn <meyerzinn@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var host string
var encoding string
var timeout int
var logLevel string
var protocol string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ipfs-cluster",
	Short: "Start and manage an IPFS cluster",
	Long: `ipfs-cluster is a tool to manage IPFS clusters.
Use "ipfs-cluster help" to list all available commands and
"ipfs-cluster help <command>" to get usage information for a
specific one.

ipfs-cluster uses the cluster API to perform requests and display
responses in a user-readable format. The location of the IPFS
Cluster server is assumed to be , but can be
configured with the --host option.

For feedback, bug reports or any additional information, visit
https://github.com/ipfs/ipfs-cluster.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global force your application.

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.ipfs-cluster.yaml)")
	RootCmd.PersistentFlags().StringVarP(&host, "host", "h","127.0.0.1:9094", "cluster host")
	RootCmd.PersistentFlags().StringVarP(&protocol, "protocol", "p", "http", "The protocol to use in connecting")
	RootCmd.PersistentFlags().StringVarP(&encoding, "encoding", "e", "text", "The encoding to use (text or json)")
	RootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 60, "The number of seconds before timeing out a request")
	RootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "info", "One of debug, info, warn, error, or fatal")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".ipfs-cluster") // name of config file (without extension)
	viper.AddConfigPath("$HOME")         // adding home directory as first search path
	viper.AutomaticEnv()                 // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func setupLogging() {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.WithError(err).Fatal("could not bind flags")
	}
	logrus.SetLevel(level)
}
