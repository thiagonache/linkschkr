package links_test

import (
	"bytes"
	"links"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidLinkIntegration(t *testing.T) {
	t.Parallel()
	// if os.Getenv("INTEGRATION_TESTS_ENABLED") == "" {
	// 	t.Skip("Set INTEGRATION_TESTS_ENABLED=true to run integration tests")
	// }
	checker := links.NewChecker("https://golang.org/",
		links.WithRunRecursively(false),
	)
	gotSuccess, gotFailures, err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{}
	if !cmp.Equal(wantFailures, gotFailures) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	}
	wantSuccess := []links.Result{
		{
			URL:        "https://golang.org/",
			Err:        nil,
			StatusCode: 200,
			ExtraSites: []string{
				"https://golang.org/",
				"https://golang.org/doc/",
				"https://golang.org/pkg/",
				"https://golang.org/project/",
				"https://golang.org/help/",
				"https://golang.org/blog/",
				"https://golang.org/dl/",
				"https://golang.org/doc/copyright.html",
				"https://golang.org/doc/tos.html",
			},
		},
	}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}

func TestValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	checker := links.NewChecker(ts.URL,
		links.WithHTTPClient(*ts.Client()),
	)
	gotSuccess, gotFailures, err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{}
	if !cmp.Equal(wantFailures, gotFailures) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	}
	wantSuccess := []links.Result{
		{
			URL:        ts.URL,
			StatusCode: 200,
			ExtraSites: nil,
		},
	}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}

func TestNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	buf := &bytes.Buffer{}
	checker := links.NewChecker(ts.URL,
		links.WithOutput(buf),
		links.WithHTTPClient(*ts.Client()),
	)
	gotSuccess, gotFailures, err := checker.Run()
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{
		{
			URL:        ts.URL,
			Err:        nil,
			StatusCode: http.StatusNotFound,
		},
	}
	if !cmp.Equal(wantFailures, gotFailures) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	}
	wantSuccess := []links.Result{}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}
