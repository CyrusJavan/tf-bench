package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/itchyny/gojq"
	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	stateFileName = "terraform.tfstate"
)

type ResourceReport struct {
	Name        string        // Name of the resource
	Count       int           // Count is the number of these resources in the workspace
	TotalTime   time.Duration // TotalTime is the time for refreshing just these resources
	AverageTime time.Duration // AverageTime is TotalTime / Count
}

type Report struct {
	Timestamp        time.Time         // Timestamp is the start of the benchmark
	TotalTime        time.Duration     // TotalTime is the duration to `terraform refresh` the entire workspace
	TerraformVersion *TerraformVersion // TerraformVersion that is running the benchmark
	Resources        []*ResourceReport // Resources is the slice of individual resource measurements
}

func NewReport() *Report {
	tv, err := terraformVersion()
	if err != nil {
		fmt.Printf("WARN: Could not find terraform version: %v\n", err)
	}
	return &Report{
		Timestamp:        time.Now(),
		TerraformVersion: tv,
	}
}

func (r *Report) String() string {
	t := table.NewWriter()
	t.SetTitle("Individual Refresh Statistics")
	t.AppendHeader(table.Row{"Type", "Count", "Refresh Time", "Refresh Time Per Resource"})
	for _, rr := range r.Resources {
		t.AppendRow(table.Row{rr.Name, rr.Count, rr.TotalTime, rr.AverageTime})
	}
	reportTemplate := `tf-bench Report %s
terraform version: v%s
provider versions:
%s
Refresh Time for Whole Workspace: %s
%s`
	tbl := t.Render()
	providerVersions := ""
	for k, v := range r.TerraformVersion.ProviderSelections {
		providerVersions += k + "=" + v + "\n"
	}
	report := fmt.Sprintf(reportTemplate, r.Timestamp.Format(time.RFC3339), r.TerraformVersion.TerraformVersion, providerVersions, r.TotalTime, tbl)
	return report
}

// ValidateEnv checks if we can run a benchmark.
func ValidateEnv() error {
	// Must be a terraform workspace, so a terraform.tfstate must exist
	if _, err := os.Stat(stateFileName); os.IsNotExist(err) {
		return fmt.Errorf("could not find state file")
	}
	tf := exec.Command("terraform", "-help")
	if err := tf.Run(); err != nil {
		return fmt.Errorf("could not execute `terraform` command")
	}
	return nil
}

func Benchmark() (*Report, error) {
	state, err := os.ReadFile(stateFileName)
	if err != nil {
		return nil, fmt.Errorf("could not read state file: %w", err)
	}
	var tfstate struct {
		Resources []struct {
			Type string
		}
	}
	err = json.Unmarshal(state, &tfstate)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal terraform state: %w", err)
	}
	// Move the resources types into a map to deduplicate and count.
	resourceTypes := map[string]int{}
	for _, r := range tfstate.Resources {
		resourceTypes[r.Type] += 1
	}

	report := NewReport()
	// Run refresh of the entire workspace to get the TotalTime
	t, err := measureRefresh(".")
	if err != nil {
		return nil, fmt.Errorf("could not measure refresh for workspace: %w", err)
	}
	report.TotalTime = t

	// Benchmark each resource type individually
	for r, count := range resourceTypes {
		rr, err := resourceBenchmark(r, state)
		if err != nil {
			return nil, fmt.Errorf("could not run individual resource benchmark for resourceType=%s: %w", r, err)
		}
		rr.Count = count
		rr.AverageTime = time.Duration(int64(rr.TotalTime) / int64(rr.Count))
		report.Resources = append(report.Resources, rr)
	}
	// Reverse sort the reports by AverageTime
	sort.Slice(report.Resources, func(i, j int) bool {
		return report.Resources[i].AverageTime > report.Resources[j].AverageTime
	})

	return report, nil
}

func resourceBenchmark(resourceType string, state []byte) (*ResourceReport, error) {
	dir := os.TempDir()
	defer os.RemoveAll(dir)
	// Copy necessary files to the temp dir
	// TODO is this portable to windows?
	cp := exec.Command("/bin/sh", "-c", fmt.Sprintf("cp -R .terraform *.tf %s", dir))
	err := cp.Run()
	if err != nil {
		return nil, fmt.Errorf("could not copy files to temp dir: %w", err)
	}
	// Change dir into the temp dir
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not get current working dir: %w", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not change dir: %w", err)
	}
	defer os.Chdir(pwd)
	// Use gojq to pull out just the resources we want to measure
	query, err := gojq.Parse(fmt.Sprintf("del(.resources[] | select(.type != \"%s\"))", resourceType))
	if err != nil {
		return nil, fmt.Errorf("could not parse gojq query: %w", err)
	}
	var s interface{}
	err = json.Unmarshal(state, &s)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal state: %w", err)
	}
	iter := query.Run(s)
	v, ok := iter.Next()
	if !ok {
		return nil, nil
	}
	if err, ok := v.(error); ok {
		return nil, fmt.Errorf("iterating through gojq query results: %w", err)
	}
	modifiedState, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshalling modified statefile: %w", err)
	}
	err = os.WriteFile(stateFileName, modifiedState, 0644)
	if err != nil {
		return nil, fmt.Errorf("writing modified state: %w", err)
	}
	// Terraform init
	initialize := exec.Command("terraform", "init")
	err = initialize.Run()
	if err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}
	// Measure terraform refresh
	t, err := measureRefresh(dir)
	if err != nil {
		return nil, fmt.Errorf("measuring refresh time: %w", err)
	}
	return &ResourceReport{
		Name:        resourceType,
		Count:       0,
		TotalTime:   t,
		AverageTime: 0,
	}, nil
}

func measureRefresh(dir string) (time.Duration, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("could not get current working dir: %w", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		return 0, fmt.Errorf("could not change dir: %w", err)
	}
	defer os.Chdir(pwd)
	c := exec.Command("terraform", "refresh")
	start := time.Now()
	err = c.Run()
	if err != nil {
		return 0, fmt.Errorf("could not run terraform refresh: %w", err)
	}
	end := time.Now()
	return end.Sub(start), nil
}

type TerraformVersion struct {
	TerraformVersion   string            `json:"terraform_version"`
	ProviderSelections map[string]string `json:"provider_selections"`
}

func terraformVersion() (*TerraformVersion, error) {
	version := exec.Command("terraform", "version", "-json")
	out, err := version.Output()
	if err != nil {
		return nil, fmt.Errorf("running terraform version -json command: %w", err)
	}
	var tv TerraformVersion
	err = json.Unmarshal(out, &tv)
	if err != nil {
		return nil, fmt.Errorf("unmarshal terraform version output: %w", err)
	}
	return &tv, nil
}
