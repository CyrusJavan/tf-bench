package bench

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBenchmark(t *testing.T) {
	tt := []struct {
		name             string
		terraformVersion string
		cfg              *Config
		workspace        []string
	}{
		{
			name:             "terraform v1.0.0, event log method",
			terraformVersion: "1.0.0",
			cfg: &Config{
				SkipControllerVersion: true,
				Iterations:            1,
				EventLog:              true,
			},
			workspace: []string{
				`
resource "random_id" "id" {
  count       = 10
  byte_length = 16
}`,
			},
		},
		{
			name:             "terraform v0.15.5, event log method",
			terraformVersion: "0.15.5",
			cfg: &Config{
				SkipControllerVersion: true,
				Iterations:            1,
				EventLog:              true,
			},
			workspace: []string{
				`
resource "random_id" "id" {
  count       = 10
  byte_length = 16
}`,
			},
		},
		{
			name:             "terraform v0.12.31",
			terraformVersion: "0.12.31",
			cfg: &Config{
				SkipControllerVersion: true,
				Iterations:            1,
				EventLog:              false,
			},
			workspace: []string{
				`
resource "random_id" "id" {
  count       = 10
  byte_length = 16
}`,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "bench.TestBenchmark.")
			require.NoError(t, err)
			defer os.RemoveAll(dir)
			err = os.Chdir(dir)
			require.NoError(t, err)
			for i, f := range tc.workspace {
				err = os.WriteFile(fmt.Sprintf("%d.tf", i), []byte(f), 0644)
				require.NoError(t, err)
			}
			terraform, err := terraformRunnerAtVersion(t, tc.terraformVersion)
			_, err = terraform.Run("init")
			require.NoError(t, err)
			_, err = terraform.Run("apply", "-auto-approve")
			require.NoError(t, err)
			report, err := Benchmark(tc.cfg, terraform)
			require.NoError(t, err)
			t.Log(report)
		})
	}
}

func terraformRunnerAtVersion(t *testing.T, v string) (*TerraformRunner, error) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	tfBenchDir := filepath.Join(home, ".tf-bench")
	os.MkdirAll(tfBenchDir, 0777)
	require.NoError(t, err)
	execPath := fmt.Sprintf("%s/terraform%s", tfBenchDir, v)
	tfRunner := &TerraformRunner{execPath: execPath}
	_, err = tfRunner.Run("version")
	if err == nil {
		return tfRunner, nil
	}
	path := fmt.Sprintf("https://releases.hashicorp.com/terraform/%[1]s/terraform_%[1]s_%[2]s_%[3]s.zip", v, runtime.GOOS, runtime.GOARCH)
	resp, err := http.Get(path)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)
	if len(zipReader.File) != 1 {
		require.Fail(t, "expected tf download to have 1 file")
	}
	unzippedFileBytes, err := readZipFile(zipReader.File[0])
	require.NoError(t, err)
	err = os.WriteFile(execPath, unzippedFileBytes, 0777)
	require.NoError(t, err)
	_, err = tfRunner.Run("version")
	if err != nil {
		return nil, fmt.Errorf("something went wrong installing tf version: %v", err)
	}
	return tfRunner, nil
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}
