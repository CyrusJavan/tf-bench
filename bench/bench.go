package bench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
)

const (
	stateFileName = "terraform.tfstate"
)

type Report struct {
	Timestamp time.Time         // Timestamp is the start of the benchmark
	TotalTime time.Duration     // TotalTime is the duration to `terraform refresh` the entire workspace
	Resources []*ResourceReport // Resources is the slice of individual resource measurements
}

func NewReport() *Report {
	return &Report{
		Timestamp: time.Now(),
	}
}

func ftime(t time.Time) string {
	return t.Format(time.RFC3339)
}

var funcMap = template.FuncMap{
	"ftime": ftime,
}

var reportTemplate = template.Must(template.New("report").Funcs(funcMap).Parse(`tf-bench report
timestamp: {{ftime .Timestamp}}
total refresh time: {{.TotalTime}}
Per resource stats
{{range .Resources}}
name: {{.Name}}
refresh time: {{.TotalTime}}
{{end}}
`))

func (r *Report) String() string {
	var b bytes.Buffer
	err := reportTemplate.Execute(&b, r)
	if err != nil {
		fmt.Printf("ERROR: failed to execute report template: %v", err)
		return ""
	}
	return b.String()
}

type ResourceReport struct {
	Name        string        // Name of the resource
	Count       int           // Count is the number of these resources in the workspace
	TotalTime   time.Duration // TotalTime is the time for refreshing just these resources
	AverageTime time.Duration // AverageTime is TotalTime / Count
}

// ValidateEnv checks if we can run a benchmark.
func ValidateEnv() error {
	// Must be a terraform workspace, so a terraform.tfstate must exist
	if _, err := os.Stat(stateFileName); os.IsNotExist(err) {
		return fmt.Errorf("could not find state file")
	}
	return nil
}

type TerraformState struct {
	Resources []struct {
		Type string
	}
}

func Benchmark() (*Report, error) {
	state, err := os.ReadFile(stateFileName)
	if err != nil {
		return nil, fmt.Errorf("could not read state file: %w", err)
	}
	var tfstate TerraformState
	err = json.Unmarshal(state, &tfstate)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal terraform state: %w", err)
	}
	// Move the resources types into a map to deduplicate and count.
	resourceTypes := map[string]bool{}
	for _, r := range tfstate.Resources {
		resourceTypes[r.Type] = true
	}

	report := NewReport()
	// Run refresh of the entire workspace to get the TotalTime
	t, err := measureRefresh(".")
	if err != nil {
		return nil, fmt.Errorf("could not measure refresh for workspace: %w", err)
	}
	report.TotalTime = t

	// Benchmark each resource type individually
	for r, _ := range resourceTypes {
		rr, err := resourceBenchmark(r, state)
		if err != nil {
			return nil, fmt.Errorf("could not run individual resource benchmark for resourceType=%s: %w", r, err)
		}
		report.Resources = append(report.Resources, rr)
	}
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
