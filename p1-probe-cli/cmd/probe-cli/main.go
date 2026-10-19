package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Whoisvox/godev/probe-cli/internal/config"
	"github.com/Whoisvox/godev/probe-cli/internal/probe"
	"github.com/spf13/cobra"
)

var (
	configPath  string
	concurrency int
	timeout     time.Duration
	maxRetries  int
	baseDelay   = 100 * time.Millisecond
	pprofAddr   string
	hold        time.Duration
)

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
				return fmt.Errorf("invalid --concurrency %d: must be >= 1", concurrency)
			}
			if timeout <= 0 {
				return fmt.Errorf("invalid --timeout %v: must be > 0", timeout)
			}
			if maxRetries < 0 {
				return fmt.Errorf("invalid --retries %d: must be >= 0", maxRetries)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// pprof debug server：探测开始前启动，用于观测运行中的 goroutine 峰值。
			// 只应绑定本机地址，不要暴露公网。
			if pprofAddr != "" {
				go func() {
					fmt.Fprintf(os.Stderr, "pprof listening on %s\n", pprofAddr)
					if err := http.ListenAndServe(pprofAddr, nil); err != nil {
						fmt.Fprintf(os.Stderr, "pprof server exited: %v\n", err)
					}
				}()
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			probeOptions := probe.ProbeOptions{
				Concurrency: concurrency,
				Timeout:     timeout,
				RetryCfg: probe.RetryConfig{
					MaxRetries: maxRetries,
					BaseDelay:  baseDelay,
				},
			}
			respSet := probe.All(ctx, cfg.Targets, probeOptions)

			data, err := json.Marshal(respSet)
			if err != nil {
				return err
			}
			// 输出契约：stdout 只有 JSON（供下游程序解析），其余一切信息走 stderr
			fmt.Println(string(data))

			// 探测完成后保持存活一段时间：供事后检查
			// （如 pprof 验证 goroutine 已回落基线），Ctrl-C 可打断
			if hold > 0 {
				fmt.Fprintf(os.Stderr, "probe done, holding %s for inspection\n", hold)
				select {
				case <-time.After(hold):
				case <-ctx.Done():
				}
			}
			return nil
		},
	}

	runCmd.Flags().StringVarP(&configPath, "config", "c", "targets.yaml", "配置文件路径")
	runCmd.Flags().IntVarP(&concurrency, "concurrency", "n", 10, "并发 worker 数量")
	runCmd.Flags().DurationVarP(&timeout, "timeout", "t", 3*time.Second, "单请求超时，必须带单位（如 2s、500ms）")
	runCmd.Flags().IntVarP(&maxRetries, "retries", "N", 2, "最大重试次数（总尝试 = N+1）")
	runCmd.Flags().StringVar(&pprofAddr, "pprof", "", "pprof debug server 监听地址（如 127.0.0.1:6060，空=关闭）")
	runCmd.Flags().DurationVar(&hold, "hold", 0, "探测完成后保持存活时长（配合 --pprof 做事后检查）")
	rootCmd.AddCommand(runCmd)
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
