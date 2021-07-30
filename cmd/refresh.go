package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/CyrusJavan/tf-bench/bench"
	"github.com/CyrusJavan/tf-bench/internal/util"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var refreshCmd = &cobra.Command{
	Use:     "refresh",
	Short:   "Measure refresh performance",
	RunE:    refreshRun,
	PreRunE: refreshPreRun,
}

func refreshRun(cmd *cobra.Command, args []string) error {
	cfg := &bench.Config{
		SkipControllerVersion: SkipControllerVersion,
		Iterations:            Iterations,
		VarFile:               VarFile,
		EventLog:              EventLog,
	}
	fmt.Printf("Starting benchmark with configuration=%+v\n", cfg)
	var logger *zap.Logger
	var err error
	if Verbose {
		logger, err = zap.NewDevelopment()
		if err != nil {
			return fmt.Errorf("could not initialize verbose logger: %w", err)
		}
	} else {
		logger, err = zap.NewProduction()
		if err != nil {
			return fmt.Errorf("could not initialize production logger: %w", err)
		}
	}
	report, err := bench.Benchmark(cfg, bench.SystemTerraform, logger)
	if err != nil {
		return err
	}
	if version == "" {
		version = "development-build"
	}
	report.BuildVersion = version
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
}

func refreshPreRun(cmd *cobra.Command, args []string) error {
	return validateEnv(SkipControllerVersion)
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
