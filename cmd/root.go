package cmd

import (
	"github.com/spf13/cobra"
)

var (
	SkipControllerVersion bool
	Iterations            int
	VarFile               string
	EventLog              bool
	Verbose               bool
	version               string
)

func init() {
	// Global flags
	rootCmd.Flags().BoolVar(&SkipControllerVersion, "skip-controller-version", false, "Skip adding controller version to generated report")
	rootCmd.Flags().BoolVarP(&Verbose, "verbose", "v", false, "Enable debug logging")
	rootCmd.Flags().StringVar(&VarFile, "var-file", "", "var-file to pass to terraform commands")

	// tf-bench version
	rootCmd.AddCommand(versionCmd)

	// tf-bench refresh
	rootCmd.AddCommand(refreshCmd)
	refreshCmd.Flags().IntVar(&Iterations, "iterations", 3, "How many times to run each refresh test. Higher number will be more accurate but slower")
	refreshCmd.Flags().BoolVar(&EventLog, "event-log", true, "Use event log method of measuring refresh")

	// tf-bench apply
	rootCmd.AddCommand(applyCmd)
}

var rootCmd = &cobra.Command{
	Use:   "tf-bench",
	Short: "tf-bench measures Terraform performance",
	Long: `
tf-bench can measure both refresh and apply performance
for the resources in your current workspace.
`,
}

func Execute() {
	_ = rootCmd.Execute()
}
