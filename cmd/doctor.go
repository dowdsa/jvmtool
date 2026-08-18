package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"jm/pkg/config"
	"jm/pkg/env"
	"jm/pkg/manager"
)

func doctorCmd() *cobra.Command {
	var verbose bool
	command := &cobra.Command{
		Use: "doctor", Short: "检查 jm、JDK、Maven 和代理配置",
		RunE: func(cmd *cobra.Command, args []string) error {
			issues := 0
			check := func(name string, ok bool, detail string) {
				status := "OK"
				if !ok {
					status = "FAIL"
					issues++
				}
				fmt.Printf("[%s] %-12s %s\n", status, name, detail)
			}
			check("root", cfg.Root != "", cfg.Root)
			check("directories", cfg.Ensure() == nil, "JDK/Maven/cache directories")
			values := env.CurrentValues(cfg.Root)
			fmt.Printf("[INFO] %-12s JAVA_HOME=%s\n", "environment", values["JAVA_HOME"])
			fmt.Printf("[INFO] %-12s M2_HOME=%s\n", "environment", values["M2_HOME"])
			for _, item := range []struct {
				kind manager.Kind
				name string
				exe  []string
			}{
				{manager.KindJDK, "jdk", []string{"bin/java", "bin/java.exe"}},
				{manager.KindMaven, "maven", []string{"bin/mvn", "bin/mvn.cmd"}},
			} {
				m := manager.NewManager(cfg, item.kind)
				installed, err := m.Installed()
				check(item.name, err == nil, fmt.Sprintf("%d installed", len(installed)))
				current, err := m.Current()
				if err != nil {
					fmt.Printf("[INFO] %-12s 未设置当前版本\n", item.name)
					continue
				}
				root := filepath.Join(cfg.Root, string(item.kind), current)
				valid := false
				for _, exe := range item.exe {
					if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(exe))); statErr == nil {
						valid = true
						break
					}
				}
				check(item.name+" current", valid, current)
			}
			if verbose {
				fmt.Printf("[INFO] %-12s %s\n", "config", cfg.SettingsPath())
				if entries, total, err := cacheEntries(); err == nil {
					fmt.Printf("[INFO] %-12s %d 个文件，占用 %s\n", "cache", len(entries), formatBytes(total))
				}
			}
			if proxy := config.ProxyURL(); proxy != nil {
				fmt.Printf("[OK] %-12s %s\n", "proxy", proxyDisplay(proxy))
			} else {
				fmt.Printf("[INFO] %-12s 未检测到代理，使用直连\n", "proxy")
			}
			if pac := config.SystemPACURL(); pac != "" {
				fmt.Printf("[WARN] %-12s 检测到 PAC：%s（当前仅支持静态系统代理）\n", "proxy PAC", pac)
			}
			if issues > 0 {
				return fmt.Errorf("doctor found %d issue(s)", issues)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&verbose, "verbose", false, "显示详细路径、配置和缓存信息")
	return command
}

func proxyDisplay(proxy *url.URL) string {
	copyURL := *proxy
	if copyURL.User != nil {
		copyURL.User = url.User(copyURL.User.Username())
	}
	return copyURL.String()
}
