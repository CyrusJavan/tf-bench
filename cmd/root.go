package cmd

import (
	"fmt"

	"github.com/CyrusJavan/tf-bench/bench"
	"github.com/spf13/cobra"
)

var (
	SkipControllerVersion bool
)

func init() {
	rootCmd.Flags().BoolVar(&SkipControllerVersion, "skip-controller-version", false, "Skip adding controller version to generated report")
}

var rootCmd = &cobra.Command{
	Use:   "tf-bench",
	Short: "tf-bench measures Terraform refresh performance",
	Long: `
tf-bench creates a report that details the Terraform refresh
performance of the current terraform workspace. 
`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return bench.ValidateEnv(SkipControllerVersion)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		report, err := bench.Benchmark(SkipControllerVersion)
		if err != nil {
			return err
		}
		fmt.Println(report)
		return nil
	},
}

func Execute() {
	_ = rootCmd.Execute()
}
