package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Output tf-bench build version",
	RunE:  versionRun,
}

func versionRun(cmd *cobra.Command, args []string) error {
	fmt.Print(version)
	return nil
}
