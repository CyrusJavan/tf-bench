package cmd

import (
	"fmt"
	"os"
	"time"

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
		reportString := report.String()
		fmt.Println(reportString)
		// Save report to file as well
		filename := "tf-bench-report-" + report.Timestamp.Format(time.RFC3339)
		err = os.WriteFile(filename, []byte(reportString), 0644)
		if err != nil {
			return fmt.Errorf("could not write report to file. The report has also been output to the console please recover the report from there: %w", err)
		}
		fmt.Printf("Wrote report to file %s\n", filename)
		return nil
	},
}

func Execute() {
	_ = rootCmd.Execute()
}
