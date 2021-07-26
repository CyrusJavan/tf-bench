package util

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
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

func ProgressWriter() progress.Writer {
	pw := progress.NewWriter()
	pw.SetAutoStop(true)
	pw.SetTrackerLength(25)
	pw.ShowOverallTracker(true)
	pw.ShowTracker(true)
	pw.ShowValue(true)
	pw.SetMessageWidth(24)
	pw.SetSortBy(progress.SortByPercentDsc)
	pw.SetStyle(progress.StyleDefault)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * 100)
	pw.Style().Options.PercentFormat = "%4.1f%%"
	return pw
}
