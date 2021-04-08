package links_test

import (
	"bytes"
	"links"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidLinkIntegration(t *testing.T) {
	t.Parallel()
	if os.Getenv("INTEGRATION_TESTS_ENABLED") == "" {
		t.Skip("Set INTEGRATION_TESTS_ENABLED=true to run integration tests")
	}
	checker := links.NewChecker("https://golang.org",
		links.WithWorkers(2),
		links.WithRunRecursively(false),
	)
	wantWorkers := 2
	gotWorkers := checker.NWorkers
	if wantWorkers != gotWorkers {
		t.Errorf("want %d workers but got %d", wantWorkers, gotWorkers)
	}
	err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantBrokenLinks := []links.Result{}
	gotBrokenLinks := checker.BrokenLinks
	if !cmp.Equal(wantBrokenLinks, gotBrokenLinks) {
		t.Errorf(cmp.Diff(wantBrokenLinks, gotBrokenLinks))
	}
}

func TestValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	checker := links.NewChecker(ts.URL,
		links.WithHTTPClient(ts.Client()),
	)
	err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantBrokenLinks := []links.Result{}
	gotBrokenLinks := checker.BrokenLinks
	if !cmp.Equal(wantBrokenLinks, gotBrokenLinks) {
		t.Errorf(cmp.Diff(wantBrokenLinks, gotBrokenLinks))
	}
}

func TestNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	buf := &bytes.Buffer{}
	checker := links.NewChecker(ts.URL,
		links.WithOutput(buf),
		links.WithHTTPClient(ts.Client()),
	)
	err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantBrokenLinks := []links.Result{
		{
			URL: ts.URL,
			Err: nil,
			StatusCode: http.StatusNotFound,
		},
	}
	gotBrokenLinks := checker.BrokenLinks
	if !cmp.Equal(wantBrokenLinks, gotBrokenLinks) {
		t.Errorf(cmp.Diff(wantBrokenLinks, gotBrokenLinks))
	}
}