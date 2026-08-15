package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"jm/internal/config"
)

var cfg = config.Default()

var rootCmd = &cobra.Command{
	Use:   "jm",
	Short: "多版本 JDK 与 Maven 管理工具",
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
	rootCmd.AddCommand(cleanCmd())
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
	group.AddCommand(
		searchCmd(name, kind),
		installCmd(name, kind),
		listCmd(name, kind),
		useCmd(name, kind),
		uninstallCmd(name, kind),
		currentCmd(name, kind),
	)
	return group
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
