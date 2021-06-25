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
	Iterations            int
)

func init() {
	rootCmd.Flags().BoolVar(&SkipControllerVersion, "skip-controller-version", false, "Skip adding controller version to generated report")
	rootCmd.Flags().IntVar(&Iterations, "iterations", 3, "How many times to run each refresh test. Higher number will be more accurate but slower")
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
		cfg := &bench.Config{
			SkipControllerVersion: SkipControllerVersion,
			Iterations:            Iterations,
		}
		fmt.Printf("Starting benchmark with configuration=%+v\n", cfg)
		report, err := bench.Benchmark(cfg)
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
