package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"jm/pkg/config"
	"jm/pkg/manager"
	"jm/pkg/update"
	"jm/pkg/version"
)

var cfg = config.Default()

var rootCmd = &cobra.Command{
	Use:     "jm",
	Short:   "多版本 JDK 与 Maven 管理工具",
	Version: version.Version,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true, // 使用自定义 completionCmd
	},
	Long: `jm 用于下载、安装并管理多版本 JDK (Temurin) 与 Maven。

默认安装根目录为 $HOME/.jvmtool，可用环境变量 JVMTOOL_HOME 覆盖。
目录结构:
  <root>/jdk/<version>/    已安装的 JDK
  <root>/maven/<version>/  已安装的 Maven
  <root>/cache/            下载缓存 (.tar.gz)
  <root>/jdk/current       指向当前 JDK 的符号链接
  <root>/maven/current     指向当前 Maven 的符号链接`,
}

func Execute() {
	cfg.LoadSettings()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfg.Root, "root", cfg.Root,
		"安装根目录 (默认 $JVMTOOL_HOME 或 $HOME/.jvmtool)")
	rootCmd.AddCommand(newToolGroup("jdk", KindJDK))
	rootCmd.AddCommand(newToolGroup("maven", KindMaven))
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(projectUseCmd())
	rootCmd.AddCommand(cleanCmd())
	rootCmd.AddCommand(cacheCmd())
	rootCmd.AddCommand(envCmd())
	rootCmd.AddCommand(updateCmd())
	rootCmd.AddCommand(doctorCmd())
	rootCmd.AddCommand(completionCmd())
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "检查并更新 jm 到最新版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()

			fmt.Printf("当前版本: %s\n", version.Version)
			rel, err := update.Latest(ctx)
			if err != nil {
				return fmt.Errorf("检查更新失败: %w", err)
			}
			latest := rel.Version()
			fmt.Printf("最新版本: %s\n", latest)

			if !update.IsNewer(version.Version, latest) {
				fmt.Println("已是最新版本。")
				return nil
			}

			fmt.Printf("发现新版本 %s\n", latest)
			executable, err := os.Executable()
			if err != nil {
				return fmt.Errorf("获取当前 CLI 路径失败: %w", err)
			}
			fmt.Println("正在下载更新...")
			downloaded, err := update.DownloadCLI(ctx, rel, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return fmt.Errorf("下载更新失败: %w", err)
			}
			if err := update.ApplyCLI(downloaded, executable); err != nil {
				return fmt.Errorf("安装更新失败: %w", err)
			}
			fmt.Println("更新成功，请重新执行 jm。")
			return nil
		},
	}
}

// Kind type alias to keep cmd decoupled from manager internals is not needed;
// we reuse the manager.Kind values via wrapper funcs below.
type kindString = string

const (
	KindJDK   kindString = "jdk"
	KindMaven kindString = "maven"
)

func newToolGroup(name, kind kindString) *cobra.Command {
	group := &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("管理 %s 版本", name),
		Long:  fmt.Sprintf("搜索、安装、切换与卸载 %s 版本。", name),
	}
	if kind == KindJDK {
		group.PersistentFlags().String("distro", "openjdk",
			"JDK 发行版 (temurin, zulu, openjdk)")
	}
	group.AddCommand(
		searchCmd(name, kind, group),
		infoCmd(name, kind, group),
		installCmd(name, kind, group),
		listCmd(name, kind),
		useCmd(name, kind),
		uninstallCmd(name, kind),
		currentCmd(name, kind),
		importCmd(name, kind),
	)
	return group
}

// getDistro extracts the --distro flag from the parent command (if present).
func getDistro(group *cobra.Command) string {
	if group == nil {
		return ""
	}
	v, _ := group.Flags().GetString("distro")
	return v
}

// newManager creates a Manager using the --distro flag when applicable.
func newManager(kind kindString, group *cobra.Command) *manager.Manager {
	return manager.NewManagerForDistro(cfg, manager.Kind(kind), getDistro(group))
}

func cleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "清理下载缓存",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Ensure(); err != nil {
				return err
			}
			if err := os.RemoveAll(cfg.CacheDir()); err != nil {
				return err
			}
			fmt.Println("已清理下载缓存:", cfg.CacheDir())
			return nil
		},
	}
}
