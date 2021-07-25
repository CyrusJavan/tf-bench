package util

import (
	"fmt"
	"os/exec"
	"time"
)

func RunCommand(name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("running command: %w output: %s", err, string(out))
	}
	return out, nil
}

func PrintSpinner(done *bool) {
	const c = `|/-\`
	var i int
	for {
		if *done {
			fmt.Print("\033[1D ")
			return
		}
		fmt.Print("\033[2D ")
		fmt.Print(string(c[i%4]))
		i++
		time.Sleep(120 * time.Millisecond)
	}
}
