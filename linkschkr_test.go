package linkschkr

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunNoRecursion(t *testing.T) {
	t.Parallel()

	wantWorkers := 2
	stdout := &bytes.Buffer{}
	checker := New([]string{"https://golang.org"},
		WithNumberWorkers(2),
		WithRunRecursively(false),
		WithOutput(stdout),
	)
	gotWorkers := checker.NWorkers
	if wantWorkers != gotWorkers {
		t.Errorf("want %d workers but got %d", wantWorkers, gotWorkers)
	}

	wantStdout := fmt.Sprintln(`Site "https://golang.org" is "up".`)
	err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	gotStdout := stdout.String()
	if !cmp.Equal(wantStdout, gotStdout) {
		t.Errorf(cmp.Diff(wantStdout, gotStdout))
	}
}
