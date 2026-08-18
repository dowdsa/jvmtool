package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func cacheCmd() *cobra.Command {
	var olderThan int
	root := &cobra.Command{Use: "cache", Short: "查看和清理下载缓存"}
	clean := &cobra.Command{
		Use: "clean", Short: "清理下载缓存",
		RunE: func(cmd *cobra.Command, args []string) error {
			if olderThan <= 0 {
				if err := os.RemoveAll(cfg.CacheDir()); err != nil {
					return err
				}
				return os.MkdirAll(cfg.CacheDir(), 0o755)
			}
			return cleanOldCache(time.Duration(olderThan) * 24 * time.Hour)
		},
	}
	root.AddCommand(
		&cobra.Command{
			Use: "list", Short: "列出下载缓存",
			RunE: func(cmd *cobra.Command, args []string) error {
				entries, total, err := cacheEntries()
				if err != nil {
					return err
				}
				if len(entries) == 0 {
					fmt.Println("没有下载缓存。")
					return nil
				}
				for _, entry := range entries {
					fmt.Printf("%-12s %s\n", formatBytes(entry.size), entry.name)
				}
				fmt.Printf("总计: %s (%d 个文件)\n", formatBytes(total), len(entries))
				return nil
			},
		},
		&cobra.Command{
			Use: "size", Short: "查看下载缓存大小",
			RunE: func(cmd *cobra.Command, args []string) error {
				_, total, err := cacheEntries()
				if err != nil {
					return err
				}
				fmt.Println(formatBytes(total))
				return nil
			},
		},
		clean,
	)
	clean.Flags().IntVar(&olderThan, "older-than", 0, "只清理超过指定天数的缓存")
	return root
}

type cacheEntry struct {
	name string
	size int64
	when time.Time
}

func cacheEntries() ([]cacheEntry, int64, error) {
	if err := cfg.Ensure(); err != nil {
		return nil, 0, err
	}
	items, err := os.ReadDir(cfg.CacheDir())
	if err != nil {
		return nil, 0, err
	}
	entries := make([]cacheEntry, 0, len(items))
	var total int64
	for _, item := range items {
		if !item.Type().IsRegular() {
			continue
		}
		info, err := item.Info()
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, cacheEntry{name: item.Name(), size: info.Size(), when: info.ModTime()})
		total += info.Size()
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].when.Before(entries[j].when) })
	return entries, total, nil
}

func cleanOldCache(age time.Duration) error {
	entries, _, err := cacheEntries()
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-age)
	for _, entry := range entries {
		if entry.when.Before(cutoff) {
			if err := os.Remove(filepath.Join(cfg.CacheDir(), entry.name)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}
