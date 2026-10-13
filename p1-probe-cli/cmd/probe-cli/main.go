package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Whoisvox/godev/probe-cli/internal/config"
	"github.com/Whoisvox/godev/probe-cli/internal/probe"
	"github.com/spf13/cobra"
)

var configPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "probe-cli",
		Short: "批量 HTTP/TCP 探测工具",
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "执行一次探测",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			var respSet []probe.Response
			// traverse cfg.Targets and call for each target probe.Probe, output json
			for _, target := range cfg.Targets {
				respSet = append(respSet, probe.Probe(target.URL))
			}
			data, err := json.Marshal(respSet)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}

	runCmd.Flags().StringVarP(&configPath, "config", "c", "targets.yaml", "配置文件路径")
	rootCmd.AddCommand(runCmd)
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
