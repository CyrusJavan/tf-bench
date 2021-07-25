package bench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AviatrixSystems/terraform-provider-aviatrix/v2/goaviatrix"
	"github.com/CyrusJavan/tf-bench/internal/util"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/itchyny/gojq"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	log "github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

const (
	stateFileName      = "terraform.tfstate"
	defaultParallelism = 10
)

var tf15 = version.Must(version.NewVersion("v0.15"))

type Config struct {
	SkipControllerVersion bool
	Iterations            int
	VarFile               string
	EventLog              bool
}

type Resource struct {
	Name  string
	Count int
}

type ResourceReport struct {
	Name      string        // Name of the resource
	Count     int           // Count is the number of these resources in the workspace
	TotalTime time.Duration // TotalTime is the time for refreshing just these resources
}

type TerraformState struct {
	Resources []struct {
		Type      string
		Instances []struct{}
	}
}

type Report struct {
	Timestamp         time.Time                   // Timestamp is the start of the benchmark
	TotalTime         time.Duration               // TotalTime is the duration to `terraform refresh` the entire workspace
	TerraformVersion  *TerraformVersion           // TerraformVersion that is running the benchmark
	ControllerVersion *goaviatrix.AviatrixVersion // ControllerVersion of the Aviatrix controller
	Resources         []*ResourceReport           // Resources is the slice of individual resource measurements
	Config            *Config                     // Config that this report was generated with
}

func (r *Report) String() string {
	t := table.NewWriter()
	t.SetTitle("Individual Resource Type Refresh Statistics")
	if r.Config.EventLog {
		t.Style().Format.Header = text.FormatDefault
		t.AppendHeader(table.Row{"Resource Type", "Count", "Average Time Per Resource", "Average*Count"})
		for _, rr := range r.Resources {
			calc := int64(rr.TotalTime) * int64(rr.Count)
			t.AppendRow(table.Row{rr.Name, rr.Count, rr.TotalTime.Round(time.Millisecond), time.Duration(calc).Round(time.Millisecond)})
		}
	} else {
		t.AppendHeader(table.Row{"Resource Type", "Count", fmt.Sprintf("Average Refresh Time of %d Measurements", r.Config.Iterations)})
		for _, rr := range r.Resources {
			t.AppendRow(table.Row{rr.Name, rr.Count, rr.TotalTime.Round(time.Millisecond)})
		}
	}

	reportTemplate := `tf-bench Report %s%s
iterations per measurement: %d%s%s
Refresh Time for Whole Workspace: %s
%s
`
	tbl := t.Render()
	providerVersions := ""
	if r.TerraformVersion != nil {
		providerVersions = "\nprovider versions:\n"
		for k, v := range r.TerraformVersion.ProviderSelections {
			providerVersions += k + "=" + v + "\n"
		}
	}
	var controllerVer string
	if r.ControllerVersion != nil {
		controllerVer = "\ncontroller version: v" + strconv.Itoa(int(r.ControllerVersion.Major)) + "." + strconv.Itoa(int(r.ControllerVersion.Minor)) + "-" + strconv.Itoa(int(r.ControllerVersion.Build))
	}
	var terraformVer string
	if r.TerraformVersion != nil {
		terraformVer = "\nterraform version: v" + r.TerraformVersion.TerraformVersion
	}
	report := fmt.Sprintf(reportTemplate, r.Timestamp.Format(time.RFC3339), controllerVer, r.Config.Iterations, terraformVer, providerVersions, r.TotalTime.Round(time.Millisecond), tbl)
	return report
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

func Benchmark(cfg *Config) (*Report, error) {
	if cfg.EventLog {
		return eventLogBenchmark(cfg)
	}
	return tempDirBenchmark(cfg)
}

func tempDirBenchmark(cfg *Config) (*Report, error) {
	tfstate, state, err := terraformState()
	if err != nil {
		return nil, err
	}

	// Move the resources types into a map to deduplicate and count.
	resourceTypes := map[string]int{}
	for _, r := range tfstate.Resources {
		resourceTypes[r.Type] += len(r.Instances)
	}

	var totalCount int
	for _, v := range resourceTypes {
		totalCount += v
	}
	fmt.Printf("Found %d resources/data_sources in the state file.\n", totalCount)

	report := NewReport(cfg)
	// Run refresh of the entire workspace to get the TotalTime
	fmt.Print("All resources measurement:  ")
	t, err := measureRefresh(".", defaultParallelism, cfg.Iterations, cfg.VarFile)
	if err != nil {
		return nil, fmt.Errorf("could not measure refresh for workspace: %w", err)
	}
	fmt.Println()
	report.TotalTime = t

	// Benchmark each resource type individually
	for r, count := range resourceTypes {
		fmt.Printf("%s measurement:  ", r)
		rr, err := resourceBenchmark(cfg, &Resource{Name: r, Count: count}, state, report.TerraformVersion)
		if err != nil {
			fmt.Printf("During the individual resource benchmark for resourceType=%s the following error occured: %v", r, err)
			continue
		}
		rr.Count = count
		report.Resources = append(report.Resources, rr)
		fmt.Println("average: " + rr.TotalTime.Round(time.Millisecond).String())
	}

	// Reverse sort the reports by TotalTime
	sort.Slice(report.Resources, func(i, j int) bool {
		return report.Resources[i].TotalTime > report.Resources[j].TotalTime
	})

	fmt.Println("Finished benchmark.")
	return report, nil
}

func eventLogBenchmark(cfg *Config) (*Report, error) {
	report := NewReport(cfg)
	if report.TerraformVersion != nil {
		v0_15_4, err1 := version.NewVersion("v0.15.4")
		v, err2 := version.NewVersion(report.TerraformVersion.TerraformVersion)
		if err1 == nil && err2 == nil && v.LessThan(v0_15_4) {
			return nil, fmt.Errorf(`terraform version is too low to use event log measurement method. 
Your terraform version is %s, event log measurement method requires at least v0.15.4.
Set --event-log=false flag to use the temporary directory measurement method.`, report.TerraformVersion.TerraformVersion)
		}
	}
	// Get the JSON event log output of a refresh
	args := []string{
		"plan",
		"-refresh-only",
		"-json",
	}
	if cfg.VarFile != "" {
		args = append(args, "-var-file="+cfg.VarFile)
	}
	var wholeWorkspaceTotal time.Duration
	reports := map[string]*ResourceReport{}
	for i := 0; i < cfg.Iterations; i++ {
		begin := time.Now()
		out, err := util.RunCommand("terraform", args...)
		finish := time.Now()
		wholeWorkspaceTotal += finish.Sub(begin)
		if err != nil {
			return nil, fmt.Errorf("running terraform plan -refresh-only -json: %w", err)
		}
		type tfEvent struct {
			Type      string
			Timestamp time.Time `json:"@timestamp"`
			Hook      struct {
				Resource struct {
					Addr         string
					ResourceType string `json:"resource_type"`
				}
			}
		}
		starts := map[string]tfEvent{}
		ends := map[string]tfEvent{}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var event tfEvent
			err := dec.Decode(&event)
			if err == io.EOF {
				// all done
				break
			}
			if err != nil {
				return nil, fmt.Errorf("decode json obj: %w", err)
			}
			if event.Type == "refresh_start" {
				starts[event.Hook.Resource.Addr] = event
			} else if event.Type == "refresh_complete" {
				ends[event.Hook.Resource.Addr] = event
			}
		}
		m := map[string][]time.Duration{}
		for k, start := range starts {
			end := ends[k]
			d := end.Timestamp.Sub(start.Timestamp)
			m[start.Hook.Resource.ResourceType] = append(m[start.Hook.Resource.ResourceType], d)
		}
		a := map[string]time.Duration{}
		for k, v := range m {
			var total int64
			for _, d := range v {
				total += int64(d)
			}
			avg := time.Duration(total / int64(len(v)))
			a[k] = avg

			rr := &ResourceReport{
				Name:      k,
				Count:     len(v),
				TotalTime: avg,
			}
			if _, ok := reports[k]; ok {
				reports[k].TotalTime += avg
			} else {
				reports[k] = rr
			}
		}
	}
	for _, r := range reports {
		r.TotalTime = time.Duration(int64(r.TotalTime) / int64(cfg.Iterations))
		report.Resources = append(report.Resources, r)
	}
	report.TotalTime = time.Duration(int64(wholeWorkspaceTotal) / int64(cfg.Iterations))

	// Reverse sort the reports by TotalTime * Count
	sort.Slice(report.Resources, func(i, j int) bool {
		return (int64(report.Resources[i].TotalTime) * int64(report.Resources[i].Count)) > (int64(report.Resources[j].TotalTime) * int64(report.Resources[j].Count))
	})

	return report, nil
}

func resourceBenchmark(cfg *Config, resource *Resource, state []byte, tfv *TerraformVersion) (*ResourceReport, error) {
	dir := os.TempDir()
	defer os.RemoveAll(dir)
	// Copy over any tfvars or tfvars.json files
	_, _ = util.RunCommand("/bin/sh", "-c", fmt.Sprintf("cp -R *.tfvars *.tfvars.json %s", dir))
	// Generate the modified TF file
	modifiedTf, err := createModifiedTerraformConfiguration(resource, cfg.VarFile, tfv)
	if err != nil {
		return nil, fmt.Errorf("creating modified tf file: %w", err)
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
	// Write the modified tf file
	err = os.WriteFile("main.tf", modifiedTf, 0644)
	if err != nil {
		return nil, fmt.Errorf("writing modified tf file: %w", err)
	}
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
	_, err = util.RunCommand("terraform", "init")
	if err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}
	// Measure terraform refresh
	t, err := measureRefresh(dir, defaultParallelism, cfg.Iterations, cfg.VarFile)
	if err != nil {
		return nil, fmt.Errorf("measuring refresh time: %w", err)
	}

	return &ResourceReport{
		Name:      resource.Name,
		TotalTime: t,
	}, nil
}

func measureRefresh(dir string, parallelism, iterations int, varFile string) (time.Duration, error) {
	// I've noticed some inflated results and it seems that
	// Terraform is doing some extra work when running an initial
	// Terraform refresh. So, we will throw out the result of the
	// first Terraform refresh.
	_, _ = measureRefreshOnce(dir, parallelism, varFile)
	var total time.Duration
	for i := 0; i < iterations; i++ {
		fmt.Printf("iteration %d:  ", i)
		var done bool
		go util.PrintSpinner(&done)
		one, err := measureRefreshOnce(dir, parallelism, varFile)
		done = true
		time.Sleep(120 * time.Millisecond)
		fmt.Print(one.Round(time.Millisecond).String() + " ")
		if err != nil {
			return 0, err
		}
		total += one
	}
	return time.Duration(int64(total) / int64(iterations)), nil
}

func measureRefreshOnce(dir string, parallelism int, varFile string) (time.Duration, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return 0, fmt.Errorf("could not get current working dir: %w", err)
	}
	err = os.Chdir(dir)
	if err != nil {
		return 0, fmt.Errorf("could not change dir: %w", err)
	}
	defer os.Chdir(pwd)
	args := []string{
		"refresh",
		fmt.Sprintf("-parallelism=%d", parallelism),
	}
	if varFile != "" {
		args = append(args, fmt.Sprintf("-var-file=%s", varFile))
	}
	start := time.Now()
	_, err = util.RunCommand("terraform", args...)
	end := time.Now()
	if err != nil {
		return 0, fmt.Errorf("could not run terraform refresh: %w", err)
	}
	return end.Sub(start), nil
}

type TerraformVersion struct {
	TerraformVersion   string            `json:"terraform_version"`
	ProviderSelections map[string]string `json:"provider_selections"`
}

var (
	simpleVersionRe = `v?(?P<version>[0-9]+(?:\.[0-9]+)*(?:-[A-Za-z0-9\.]+)?)`

	versionOutputRe         = regexp.MustCompile(`^Terraform ` + simpleVersionRe)
	providerVersionOutputRe = regexp.MustCompile(`(\n\+ provider[\. ](?P<name>\S+) ` + simpleVersionRe + `)`)
)

func terraformVersion() (*TerraformVersion, error) {
	out, err := util.RunCommand("terraform", "version", "-json")
	if err != nil {
		return nil, fmt.Errorf("running terraform version -json command: %w", err)
	}
	var tv TerraformVersion
	err = json.Unmarshal(out, &tv)
	if err != nil {
		// Couldn't unmarshal, could be on old Terraform that does not
		// support -json output.
		v, pv, err := parseOldVersionOutput(string(out))
		if err != nil {
			return nil, fmt.Errorf("parsing terraform version output: %w", err)
		}
		pvs := map[string]string{}
		for k, v := range pv {
			pvs[k] = v.String()
		}
		tv = TerraformVersion{
			TerraformVersion:   v.String(),
			ProviderSelections: pvs,
		}
	}
	return &tv, nil
}

// From: github.com/hashicorp/terraform-exec/tfexec/version.go
func parseOldVersionOutput(stdout string) (*version.Version, map[string]*version.Version, error) {
	stdout = strings.TrimSpace(stdout)

	submatches := versionOutputRe.FindStringSubmatch(stdout)
	if len(submatches) != 2 {
		return nil, nil, fmt.Errorf("unexpected number of version matches %d for %s", len(submatches), stdout)
	}
	v, err := version.NewVersion(submatches[1])
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse version %q: %w", submatches[1], err)
	}

	allSubmatches := providerVersionOutputRe.FindAllStringSubmatch(stdout, -1)
	provV := map[string]*version.Version{}

	for _, submatches := range allSubmatches {
		if len(submatches) != 4 {
			return nil, nil, fmt.Errorf("unexpected number of providerion version matches %d for %s", len(submatches), stdout)
		}

		v, err := version.NewVersion(submatches[3])
		if err != nil {
			return nil, nil, fmt.Errorf("unable to parse provider version %q: %w", submatches[3], err)
		}

		provV[submatches[2]] = v
	}

	return v, provV, err
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

func terraformState() (*TerraformState, []byte, error) {
	state, err := util.RunCommand("terraform", "state", "pull")
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

func createModifiedTerraformConfiguration(resource *Resource, varFile string, tfVersion *TerraformVersion) ([]byte, error) {
	// We want to build a tf file that contains just these block types:
	// variable
	// provider
	// terraform
	tfFiles, err := filepath.Glob("*.tf")
	if err != nil {
		fmt.Printf("WARN filepath.Glob: %v\n", err)
	}
	modifiedTfFile := hclwrite.NewEmptyFile()
	for _, name := range tfFiles {
		fileContent, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("could not read tf file %s: %w", name, err)
		}
		f, diags := hclwrite.ParseConfig(fileContent, name, hcl.InitialPos)
		if diags.HasErrors() {
			return nil, fmt.Errorf("parsing tf file %s: %s", name, diags.Error())
		}
		blocks := f.Body().Blocks()
		for _, block := range blocks {
			if block.Type() == "variable" || block.Type() == "provider" || block.Type() == "terraform" {
				if block.Type() == "terraform" {
					tfBlocks := block.Body().Blocks()
					for _, tfBlock := range tfBlocks {
						// Remove the backend block to force the
						// use of the local tfstate file.
						if tfBlock.Type() == "backend" {
							block.Body().RemoveBlock(tfBlock)
						}
						// Remove unnecessary required_providers
						if tfBlock.Type() == "required_providers" {
							for k, _ := range tfBlock.Body().Attributes() {
								if k != strings.Split(resource.Name, "_")[0] {
									tfBlock.Body().RemoveAttribute(k)
								}
							}
						}
					}
				}
				tfv, err := version.NewVersion(tfVersion.TerraformVersion)
				if err != nil {
					tfv = version.Must(version.NewVersion("v0.12"))
				}
				if block.Type() == "provider" && tfv.GreaterThanOrEqual(tf15) {
					if labels := block.Labels(); len(labels) > 0 {
						label := labels[0]
						if label != strings.Split(resource.Name, "_")[0] {
							continue
						}
						attrs := block.Body().Attributes()
						for k, v := range attrs {
							if len(v.Expr().Variables()) == 0 {
								continue
							}
							block.Body().SetAttributeValue(k, evaluate(v, varFile))
						}
					}
				}
				modifiedTfFile.Body().AppendBlock(block)
			}
		}
	}
	return modifiedTfFile.Bytes(), nil
}

func evaluate(attr *hclwrite.Attribute, varFile string) cty.Value {
	return eval(attr, varFile, false)
}

func eval(attr *hclwrite.Attribute, varFile string, sensitive bool) cty.Value {
	args := []string{
		"console",
	}
	if varFile != "" {
		args = append(args, fmt.Sprintf("-var-file=%s", varFile))
	}
	console := exec.Command("terraform", args...)
	pipe, _ := console.StdinPipe()

	var b bytes.Buffer
	console.Stdout = &b
	err := console.Start()
	if err != nil {
		fmt.Println(err)
	}
	attrString := string(attr.Expr().BuildTokens(nil).Bytes())
	if sensitive {
		attrString = "nonsensitive(" + attrString + ")"
	}
	_, err = io.WriteString(pipe, attrString)
	if err != nil {
		fmt.Println(err)
	}
	pipe.Close()
	err = console.Wait()
	if err != nil {
		fmt.Println(err)
	}
	s := b.String()
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)
	if s == "(sensitive)" && !sensitive {
		return eval(attr, varFile, true)
	}
	return cty.StringVal(s)
}
