package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

func newCallCmd() *cobra.Command {
	var prompt, appName, title string
	var digits int

	cmd := &cobra.Command{
		Use:   "call",
		Short: "Enqueue a DTMF call via the local API",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := NewClient("").CreateCall(CreateCallInput{
				Prompt:    prompt,
				App:       appName,
				Title:     title,
				MaxDigits: digits,
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"id": id})
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt read to the agent")
	cmd.Flags().StringVar(&appName, "app", "", "Focus target application name")
	cmd.Flags().StringVar(&title, "title", "", "Optional window title substring")
	cmd.Flags().IntVar(&digits, "digits", 1, "Max DTMF digits to collect")
	_ = cmd.MarkFlagRequired("prompt")
	_ = cmd.MarkFlagRequired("app")
	return cmd
}
