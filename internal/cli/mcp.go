package cli

import (
	"context"

	accmcp "github.com/MRsagi/agents-call-center/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	var baseURL string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP stdio server proxying to the local acc HTTP API",
		Long:  "Requires a running daemon (acc serve). Tools call the local HTTP API; use ACC_URL or --url to override the default bind address.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return accmcp.Run(context.Background(), baseURL)
		},
	}
	cmd.Flags().StringVar(&baseURL, "url", "", "ACC HTTP base URL (default: ACC_URL or http://127.0.0.1:8741)")
	return cmd
}
