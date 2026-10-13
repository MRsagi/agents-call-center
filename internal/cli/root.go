package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "acc",
		Short: "Agents call center daemon and client",
	}
	root.AddCommand(
		newServeCmd(),
		newCallCmd(),
		newStatusCmd(),
		newVoicemailCmd(),
	)
	return root
}

func Execute() error {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
