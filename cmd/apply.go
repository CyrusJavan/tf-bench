package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/CyrusJavan/tf-bench/bench"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var applyCmd = &cobra.Command{
	Use:     "apply",
	Short:   "Measure apply performance",
	RunE:    applyRun,
	PreRunE: applyPreRun,
}

func applyRun(cmd *cobra.Command, args []string) error {
	cfg := &bench.Config{
		SkipControllerVersion: SkipControllerVersion,
		VarFile:               VarFile,
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
	report, err := bench.ApplyBenchmark(cfg, bench.SystemTerraform, logger)
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
	filename := "tf-bench-apply-report-" + report.Timestamp.Format(time.RFC3339)
	err = os.WriteFile(filename, []byte(reportString), 0644)
	if err != nil {
		return fmt.Errorf("could not write report to file. The report has also been output to the console please recover the report from there: %w", err)
	}
	fmt.Printf("Wrote report to file %s\n", filename)
	return nil
}

func applyPreRun(cmd *cobra.Command, args []string) error {
	return validateEnv(SkipControllerVersion)
}
