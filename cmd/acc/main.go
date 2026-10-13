package main

import (
	"os"

	"github.com/MRsagi/agents-call-center/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
