package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Whoisvox/godev/probe-cli/internal/config"
	"github.com/Whoisvox/godev/probe-cli/internal/probe"
	"github.com/spf13/cobra"
)

var configPath string
var concurrency int
var timeout time.Duration
var maxRetries int
var baseDelay time.Duration = 100 * time.Millisecond

func main() {
	rootCmd := &cobra.Command{
		Use:   "probe-cli",
		Short: "批量 HTTP/TCP 探测工具",
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "执行一次探测",
		RunE: func(cmd *cobra.Command, args []string) error {
			if concurrency < 1 {
				return fmt.Errorf("invalid -- concurrency %d: must be >= 1", concurrency)
			}
			if timeout <= 0 {
				return fmt.Errorf("invalid -- timeout %v: must be > 0", timeout)
			}
			if maxRetries < 0 {
				return fmt.Errorf("invalid -- maxRetries %d: must be >= 0", maxRetries)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			retryCfg := probe.RetryConfig{
				MaxRetries: maxRetries,
				BaseDelay:  baseDelay,
			}
			probeOptions := probe.ProbeOptions{
				Concurrency: concurrency,
				Timeout:     timeout,
				RetryCfg:    retryCfg,
			}
			// traverse cfg.Targets and call for each target probe.Probe, output json
			respSet := probe.All(ctx, cfg.Targets, probeOptions)
			data, err := json.Marshal(respSet)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}

	runCmd.Flags().StringVarP(&configPath, "config", "c", "targets.yaml", "配置文件路径")
	runCmd.Flags().IntVarP(&concurrency, "concurrency", "n", 10, "并发数量")
	runCmd.Flags().DurationVarP(&timeout, "timeout", "t", 3*time.Second, "单个请求超时时间")
	runCmd.Flags().IntVarP(&maxRetries, "retries", "N", 2, "重试次数")
	rootCmd.AddCommand(runCmd)
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
