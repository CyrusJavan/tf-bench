package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/CyrusJavan/tf-bench/bench"
	"github.com/CyrusJavan/tf-bench/internal/util"
	"github.com/spf13/cobra"
)

var (
	SkipControllerVersion bool
	Iterations            int
	VarFile               string
	EventLog              bool
)

func init() {
	rootCmd.Flags().BoolVar(&SkipControllerVersion, "skip-controller-version", false, "Skip adding controller version to generated report")
	rootCmd.Flags().BoolVar(&EventLog, "event-log", true, "Use event log method of measuring refresh")
	rootCmd.Flags().IntVar(&Iterations, "iterations", 3, "How many times to run each refresh test. Higher number will be more accurate but slower")
	rootCmd.Flags().StringVar(&VarFile, "var-file", "", "var-file to pass to terraform commands")
}

var rootCmd = &cobra.Command{
	Use:   "tf-bench",
	Short: "tf-bench measures Terraform refresh performance",
	Long: `
tf-bench creates a report that details the Terraform refresh
performance of the current terraform workspace. 
`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateEnv(SkipControllerVersion)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &bench.Config{
			SkipControllerVersion: SkipControllerVersion,
			Iterations:            Iterations,
			VarFile:               VarFile,
			EventLog:              EventLog,
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

// validateEnv checks if we can run a benchmark.
func validateEnv(skipControllerVersion bool) error {
	// Must be able to execute terraform binary
	_, err := util.RunCommand("terraform", "-help")
	if err != nil {
		return fmt.Errorf("could not execute `terraform` command")
	}
	// Need Aviatrix environment variables as well if not skipping controller version
	if !skipControllerVersion {
		requiredEnvVars := []string{
			"AVIATRIX_CONTROLLER_IP",
			"AVIATRIX_USERNAME",
			"AVIATRIX_PASSWORD",
		}
		for _, v := range requiredEnvVars {
			if s := os.Getenv(v); s == "" {
				return fmt.Errorf(`environment variable %s is not set. 
The environment variables %v must be set to include the controller version in the generated report. 
Set --skip-controller-version flag to skip including controller version in the report.`, v, requiredEnvVars)
			}
		}
	}
	return nil
}

func Execute() {
	_ = rootCmd.Execute()
}
