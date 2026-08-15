package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"jm/internal/env"
)

func envCmd() *cobra.Command {
	group := &cobra.Command{
		Use:   "env",
		Short: "管理环境变量配置",
		Long:  "查看或清理 shell 中由 jm 写入的环境变量配置 (JAVA_HOME/M2_HOME 等)。",
	}
	group.AddCommand(envShowCmd(), envCleanCmd())
	return group
}

func envShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "显示当前环境变量配置",
		RunE: func(cmd *cobra.Command, args []string) error {
			rcFile := env.RCFile()
			fmt.Printf("Shell 配置文件: %s\n", rcFile)
			if env.HasBlock(rcFile) {
				fmt.Println("环境变量块: 已配置")
			} else {
				fmt.Println("环境变量块: 未配置")
			}
			values := env.CurrentValues(cfg.Root)
			fmt.Println("\n当前变量值:")
			for _, k := range []string{"JVMTOOL_HOME", "JAVA_HOME", "M2_HOME", "MAVEN_HOME"} {
				fmt.Printf("  %s=%s\n", k, values[k])
			}
			if env.IsWindows() {
				fmt.Println("\n提示: Windows 环境变量在用户注册表，请使用 'jm env clean' 或手动清理。")
			}
			return nil
		},
	}
}

func envCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "从 shell 配置中移除 jm 环境变量块",
		RunE: func(cmd *cobra.Command, args []string) error {
			rcFile := env.RCFile()
			changed, err := env.RemoveBlock(rcFile)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("已从 %s 移除 jm 环境变量块\n", rcFile)
				fmt.Println("提示: 重新加载 shell 后 JAVA_HOME/M2_HOME 将失效。")
			} else {
				fmt.Printf("%s 中没有 jm 环境变量块\n", rcFile)
			}
			return nil
		},
	}
}
