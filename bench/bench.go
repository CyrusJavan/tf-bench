package bench

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"time"

	"github.com/AviatrixSystems/terraform-provider-aviatrix/v2/goaviatrix"
	"github.com/itchyny/gojq"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
)

const (
	stateFileName      = "terraform.tfstate"
	defaultParallelism = 10
	clearLine          = "\033[2K"
)

type Config struct {
	SkipControllerVersion bool
	Iterations            int
}

type ResourceReport struct {
	Name      string        // Name of the resource
	Count     int           // Count is the number of these resources in the workspace
	TotalTime time.Duration // TotalTime is the time for refreshing just these resources
}

type Report struct {
	Timestamp         time.Time                   // Timestamp is the start of the benchmark
	TotalTime         time.Duration               // TotalTime is the duration to `terraform refresh` the entire workspace
	TerraformVersion  *TerraformVersion           // TerraformVersion that is running the benchmark
	ControllerVersion *goaviatrix.AviatrixVersion // ControllerVersion of the Aviatrix controller
	Resources         []*ResourceReport           // Resources is the slice of individual resource measurements
	Config            *Config                     // Config that this report was generated with
}

type Resource struct {
	Name  string
	Count int
}

func NewReport(cfg *Config) *Report {
	tv, err := terraformVersion()
	if err != nil {
		fmt.Printf("WARN: Could not find terraform version: %v\n", err)
	}
	report := Report{
		Timestamp:        time.Now(),
		TerraformVersion: tv,
		Config:           cfg,
	}
	if !cfg.SkipControllerVersion {
		av, err := controllerVersion()
		if err != nil {
			fmt.Printf("WARN: Could not find controller version: %v\n", err)
		}
		report.ControllerVersion = av
	}
	return &report
}

func (r *Report) String() string {
	t := table.NewWriter()
	t.SetTitle("Individual Refresh Statistics")
	t.AppendHeader(table.Row{"Type", "Count", "Refresh Time"})
	for _, rr := range r.Resources {
		t.AppendRow(table.Row{rr.Name, rr.Count, rr.TotalTime})
	}
	reportTemplate := `tf-bench Report %s%s
iterations per measurement: %d
terraform version: v%s
provider versions:
%s
Refresh Time for Whole Workspace: %s
%s
`
	tbl := t.Render()
	providerVersions := ""
	for k, v := range r.TerraformVersion.ProviderSelections {
		providerVersions += k + "=" + v + "\n"
	}
	var controllerVer string
	if r.ControllerVersion != nil {
		controllerVer = "\ncontroller version: v" + strconv.Itoa(int(r.ControllerVersion.Major)) + "." + strconv.Itoa(int(r.ControllerVersion.Minor)) + "-" + strconv.Itoa(int(r.ControllerVersion.Build))
	}
	report := fmt.Sprintf(reportTemplate, r.Timestamp.Format(time.RFC3339), controllerVer, r.Config.Iterations, r.TerraformVersion.TerraformVersion, providerVersions, r.TotalTime, tbl)
	return report
}

// ValidateEnv checks if we can run a benchmark.
func ValidateEnv(skipControllerVersion bool) error {
	// Must be a terraform workspace, so a terraform.tfstate must exist
	if _, err := os.Stat(stateFileName); os.IsNotExist(err) {
		return fmt.Errorf("could not find state file")
	}
	// Must be able to execute terraform binary
	_, err := runCommand("terraform", "-help")
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

func Benchmark(cfg *Config) (*Report, error) {
	tfstate, state, err := terraformState()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Found %d resources/data_sources in the state file.\n", len(tfstate.Resources))

	// Move the resources types into a map to deduplicate and count.
	resourceTypes := map[string]int{}
	for _, r := range tfstate.Resources {
		resourceTypes[r.Type] += len(r.Instances)
	}

	report := NewReport(cfg)
	// Run refresh of the entire workspace to get the TotalTime
	fmt.Print("Starting measurement for whole workspace refresh.")
	t, err := measureRefresh(".", defaultParallelism, cfg.Iterations)
	if err != nil {
		return nil, fmt.Errorf("could not measure refresh for workspace: %w", err)
	}
	report.TotalTime = t
	fmt.Print(clearLine)
	fmt.Print("\r")
	fmt.Println("Finished measurement for whole workspace refresh.")

	// Benchmark each resource type individually
	for r, count := range resourceTypes {
		fmt.Printf("Starting measurement for individual resource %q refresh.", r)
		rr, err := resourceBenchmark(cfg, &Resource{Name: r, Count: count}, state)
		if err != nil {
			return nil, fmt.Errorf("could not run individual resource benchmark for resourceType=%s: %w", r, err)
		}
		rr.Count = count
		report.Resources = append(report.Resources, rr)
		fmt.Print(clearLine)
		fmt.Print("\r")
		fmt.Printf("Finished measurement for individual resource %q refresh.\n", r)
	}

	// Reverse sort the reports by TotalTime
	sort.Slice(report.Resources, func(i, j int) bool {
		return report.Resources[i].TotalTime > report.Resources[j].TotalTime
	})

	fmt.Println("Finished benchmark.")
	return report, nil
}

func resourceBenchmark(cfg *Config, resource *Resource, state []byte) (*ResourceReport, error) {
	dir := os.TempDir()
	defer os.RemoveAll(dir)
	// Copy necessary files to the temp dir
	// TODO is this portable to windows?
	_, err := runCommand("/bin/sh", "-c", fmt.Sprintf("cp -R .terraform *.tf %s", dir))
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
	query, err := gojq.Parse(fmt.Sprintf("del(.resources[] | select(.type != \"%s\"))", resource.Name))
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
	_, err = runCommand("terraform", "init")
	if err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}
	// Measure terraform refresh
	t, err := measureRefresh(dir, resource.Count, cfg.Iterations)
	if err != nil {
		return nil, fmt.Errorf("measuring refresh time: %w", err)
	}
	return &ResourceReport{
		Name:      resource.Name,
		TotalTime: t,
	}, nil
}

func measureRefresh(dir string, parallelism, iterations int) (time.Duration, error) {
	var total time.Duration
	for i := 0; i < iterations; i++ {
		one, err := measureRefreshOnce(dir, parallelism)
		if err != nil {
			return 0, err
		}
		total += one
	}
	return time.Duration(int64(total) / int64(iterations)), nil
}

func measureRefreshOnce(dir string, parallelism int) (time.Duration, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("could not get current working dir: %w", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		return 0, fmt.Errorf("could not change dir: %w", err)
	}
	defer os.Chdir(pwd)
	start := time.Now()
	_, err = runCommand("terraform", "refresh", fmt.Sprintf("-parallelism=%d", parallelism))
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
	out, err := runCommand("terraform", "version", "-json")
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

func controllerVersion() (*goaviatrix.AviatrixVersion, error) {
	username := os.Getenv("AVIATRIX_USERNAME")
	password := os.Getenv("AVIATRIX_PASSWORD")
	ip := os.Getenv("AVIATRIX_CONTROLLER_IP")
	log.SetOutput(ioutil.Discard)
	client, err := goaviatrix.NewClient(username, password, ip, nil)
	if err != nil {
		return nil, fmt.Errorf("could not initialize aviatrix client: %w", err)
	}
	_, version, err := client.GetCurrentVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get controller version: %w", err)
	}
	return version, nil
}

func runCommand(name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running command: %w output: %s", err, string(out))
	}
	return out, nil
}

type TerraformState struct {
	Resources []struct {
		Type      string
		Instances []struct{}
	}
}

func terraformState() (*TerraformState, []byte, error) {
	state, err := os.ReadFile(stateFileName)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read state file: %w", err)
	}
	var tfstate TerraformState
	err = json.Unmarshal(state, &tfstate)
	if err != nil {
		return nil, nil, fmt.Errorf("could not unmarshal terraform state: %w", err)
	}
	return &tfstate, state, nil
}
