package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"jm/pkg/download"
)

func infoCmd(name, kind kindString, group *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "info [版本]",
		Short: "查看指定版本的详细信息（大小、校验和、镜像）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m := newManager(kind, group)
			art, err := m.Resolve(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("%-12s %s\n", "Version:", art.Version)
			fmt.Printf("%-12s %s\n", "Filename:", art.Filename)
			if art.Size > 0 {
				fmt.Printf("%-12s %s\n", "Size:", download.HumanSize(art.Size))
			}
			if art.SHA256 != "" {
				fmt.Printf("%-12s %s\n", "SHA256:", art.SHA256)
			} else if art.SHA512 != "" {
				fmt.Printf("%-12s %s\n", "SHA512:", art.SHA512)
			} else {
				fmt.Printf("%-12s (none)\n", "Checksum:")
			}
			fmt.Printf("%-12s %s\n", "URL:", art.URL)
			for _, mirror := range art.Mirrors {
				fmt.Printf("%-12s %s\n", "Mirror:", mirror)
			}
			return nil
		},
	}
}
