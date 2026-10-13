package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show API health and queue summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			sum, err := NewClient("").Status()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(sum)
		},
	}
}
