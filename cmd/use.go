package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"jm/pkg/manager"
	"jm/pkg/project"
)

// projectUseCmd implements `jm use` (no subcommand) which reads .jvmtoolrc
// from the current directory (or any parent) and switches JDK/Maven versions.
func projectUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use",
		Short: "根据 .jvmtoolrc 切换当前项目的 JDK 和 Maven 版本",
		Long: `从当前目录向上查找 .jvmtoolrc 文件，根据其中指定的版本一次性切换 JDK 和 Maven。

.jvmtoolrc 示例:
  jdk=17
  maven=3.9.11`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rc := project.FindFromCwd()
			if rc == nil {
				return fmt.Errorf("当前目录及上级目录中未找到 .jvmtoolrc 文件")
			}
			fmt.Printf("读取 %s/.jvmtoolrc\n", rc.Dir)
			switched := false
			if rc.JDK != "" {
				m := manager.NewManager(cfg, manager.KindJDK)
				exact, err := m.Use(rc.JDK)
				if err != nil {
					fmt.Fprintf(os.Stderr, "JDK 切换失败: %v\n", err)
				} else {
					fmt.Printf("  JDK    → %s\n", exact)
					switched = true
				}
			}
			if rc.Maven != "" {
				m := manager.NewManager(cfg, manager.KindMaven)
				exact, err := m.Use(rc.Maven)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Maven 切换失败: %v\n", err)
				} else {
					fmt.Printf("  Maven  → %s\n", exact)
					switched = true
				}
			}
			if !switched {
				return fmt.Errorf("没有成功切换任何版本")
			}
			fmt.Println("\n提示: source ~/.bashrc 或重新打开终端使环境变量生效")
			return nil
		},
	}
}
