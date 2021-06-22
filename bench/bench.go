package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type Report struct {
	Timestamp time.Time        // Timestamp is the start of the benchmark
	TotalTime time.Duration    // TotalTime is the duration to `terraform refresh` the entire workspace
	Resources []ResourceReport // Resources is the slice of individual resource measurements
}

func (r *Report) String() string {
	// TODO nice report formatting
	return r.Timestamp.String()
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
	if _, err := os.Stat("terraform.tfstate"); os.IsNotExist(err) {
		return fmt.Errorf("could not find terraform.tfstate file")
	}
	return nil
}

type TerraformState struct {
	Values struct{
		RootModule struct{
			Resources []struct{
				Type string
			}
		} `json:"root_module"`
	}
}

func Benchmark() (*Report, error) {
	c := exec.Command("terraform", "show", "-json")
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("could not run `terraform show -json`: %w", err)
	}
	var tfstate TerraformState
	err = json.Unmarshal(out, &tfstate)
	if err != nil {
		return nil, fmt.Errorf("coudl not unmarshal terraform state: %w", err)
	}
	fmt.Println(tfstate.Values.RootModule.Resources)
	return nil, nil
}
