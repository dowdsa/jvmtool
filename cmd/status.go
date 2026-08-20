package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"jm/pkg/download"
	"jm/pkg/manager"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "显示 JDK、Maven 和缓存的全局状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cfg.Ensure(); err != nil {
				return err
			}
			for _, kind := range []manager.Kind{manager.KindJDK, manager.KindMaven} {
				m := manager.NewManager(cfg, kind)
				installed, err := m.Installed()
				if err != nil {
					return err
				}
				current, _ := m.Current()
				label := string(kind)
				if current != "" {
					fmt.Printf("%-8s %-20s (%d installed)\n", label, current, len(installed))
				} else if len(installed) > 0 {
					fmt.Printf("%-8s %-20s (%d installed)\n", label, "(none selected)", len(installed))
				} else {
					fmt.Printf("%-8s %-20s\n", label, "(not installed)")
				}
			}
			entries, total, err := cacheEntries()
			if err != nil {
				return err
			}
			fmt.Printf("%-8s %-20s (%d files)\n", "cache", download.HumanSize(total), len(entries))
			fmt.Printf("%-8s %s\n", "home", cfg.Root)
			return nil
		},
	}
}
