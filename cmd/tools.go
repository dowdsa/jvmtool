package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"jm/internal/config"
	"jm/internal/manager"
)

func searchCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "search [版本关键字]",
		Short: "搜索可下载的可用版本",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			m := manager.NewManager(cfg, manager.Kind(kind))
			versions, err := m.Search(cmd.Context(), query, 0)
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				fmt.Printf("没有找到匹配的 %s 版本\n", name)
				return nil
			}
			fmt.Printf("可用的 %s 版本 (%d 个):\n", name, len(versions))
			for _, v := range versions {
				fmt.Printf("  %s\n", v)
			}
			return nil
		},
	}
}

func installCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "install <版本>",
		Short: "下载并安装指定版本",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := manager.NewManager(cfg, manager.Kind(kind))
			art, err := m.Install(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("提示: 运行 '%s %s use %s' 切换到此版本\n", os.Args[0], name, art.Version)
			return nil
		},
	}
}

func listCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出已安装的版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := manager.NewManager(cfg, manager.Kind(kind))
			installed, err := m.Installed()
			if err != nil {
				return err
			}
			current, _ := m.Current()
			if len(installed) == 0 {
				fmt.Printf("尚未安装任何 %s 版本\n", name)
				return nil
			}
			fmt.Printf("已安装的 %s 版本 (%d 个):\n", name, len(installed))
			for _, v := range installed {
				mark := " "
				if v == current {
					mark = "*"
				}
				fmt.Printf("  %s %s%s\n", mark, v, func() string {
					if v == current {
						return " (当前)"
					}
					return ""
				}())
			}
			return nil
		},
	}
}

func useCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "use <版本>",
		Short: "切换当前使用版本",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := manager.NewManager(cfg, manager.Kind(kind))
			exact, err := m.Use(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("已切换到 %s %s\n", name, exact)
			printEnvHint(name, m.Cfg, exact)
			return nil
		},
	}
}

func uninstallCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <版本>",
		Short: "卸载指定版本",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := manager.NewManager(cfg, manager.Kind(kind))
			if err := m.Uninstall(args[0]); err != nil {
				return err
			}
			fmt.Printf("已卸载 %s %s\n", name, args[0])
			return nil
		},
	}
}

func currentCmd(name, kind kindString) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "显示当前使用版本",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := manager.NewManager(cfg, manager.Kind(kind))
			cur, err := m.Current()
			if err != nil {
				fmt.Printf("当前没有设置 %s 版本 (使用 '%s %s use <版本>' 切换)\n", name, os.Args[0], name)
				return nil
			}
			fmt.Printf("%s 当前版本: %s\n", name, cur)
			printEnvHint(name, m.Cfg, cur)
			return nil
		},
	}
}

func printEnvHint(name string, c *config.Config, version string) {
	root := c.Root
	switch name {
	case "jdk":
		home := filepath.Join(root, "jdk", version)
		fmt.Printf("\n设置环境变量:\n")
		fmt.Printf("  export JAVA_HOME=%s\n", home)
		fmt.Printf("  export PATH=$JAVA_HOME/bin:$PATH\n")
	case "maven":
		home := filepath.Join(root, "maven", version)
		fmt.Printf("\n设置环境变量:\n")
		fmt.Printf("  export M2_HOME=%s\n", home)
		fmt.Printf("  export MAVEN_HOME=%s\n", home)
		fmt.Printf("  export PATH=$M2_HOME/bin:$PATH\n")
	}
}
