package links_test

import (
	"io"
	"links"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestValidLinkIntegration(t *testing.T) {
	t.Parallel()
	testURL := os.Getenv("LINKSCHKR_TESTS_url")
	if testURL == "" {
		t.Skip("Set LINKSCHKR_TESTS_url=<url to check> to run integration tests")
	}

	gotFailures := links.Check(testURL,
		links.WithRecursive(false),
		links.WithStdout(io.Discard),
	)
	wantFailures := []*links.Result{}
	if !cmp.Equal(wantFailures, gotFailures) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	}
}

func TestValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))

	gotFailures := links.Check(ts.URL,
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
	)
	wantFailures := []*links.Result{}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors()) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors()))
	}
}

func TestNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	gotFailures := links.Check(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
	)
	result := links.NewResult(ts.URL, "")
	result.SetStatus("down", http.StatusNotFound)
	wantFailures := []*links.Result{result}

	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors(), cmp.AllowUnexported(links.Result{})) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors(), cmp.AllowUnexported(links.Result{})))
	}
}
