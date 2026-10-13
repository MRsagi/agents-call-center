package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

func newVoicemailCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "voicemail",
		Short: "Show voicemail summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			vm, err := NewClient("").Voicemail()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(vm)
		},
	}
}
