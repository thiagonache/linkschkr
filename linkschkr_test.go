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

	gotSuccess, gotFailures := links.Run(testURL,
		links.WithRecursive(false),
		links.WithStdout(io.Discard),
	)
	wantFailures := []*links.Result{}
	if !cmp.Equal(wantFailures, gotFailures) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	}
	wantSuccess := []*links.Result{
		{
			URL:          testURL,
			State:        "up",
			Error:        nil,
			ResponseCode: 200,
		},
	}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}

func TestValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))

	gotSuccess, gotFailures := links.Run(ts.URL,
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
	)
	wantFailures := []*links.Result{}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors()) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors()))
	}
	wantSuccess := []*links.Result{
		{
			URL:          ts.URL,
			ResponseCode: 200,
			State:        "up",
		},
	}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}

func TestNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusNotFound)
	}))

	gotSuccess, gotFailures := links.Run(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
	)
	wantFailures := []*links.Result{
		{
			URL:          ts.URL,
			ResponseCode: http.StatusNotFound,
			State:        "down",
		},
	}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors()) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors()))
	}
	wantSuccess := []*links.Result{}
	if !cmp.Equal(wantSuccess, gotSuccess) {
		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	}
}
