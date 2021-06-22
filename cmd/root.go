package cmd

import (
	"fmt"
	"os"

	"github.com/CyrusJavan/tf-bench/bench"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tf-bench",
	Short: "tf-bench measures Terraform refresh performance",
	Long: `
tf-bench creates a report that details the Terraform refresh
performance of the current terraform workspace. 
`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return bench.ValidateEnv()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := bench.Benchmark()
		if err != nil {
			return err
		}
		fmt.Println(report)
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
