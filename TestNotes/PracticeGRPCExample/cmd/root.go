package cmd

import (
	"MLC_GO/TestNotes/GenPracticeExample/pkg/logging"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command {
	Use: "grpc",
	Short: "Run the gRPC hello-world server",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logging.Info(err)

		os.Exit(-1)
	}
}