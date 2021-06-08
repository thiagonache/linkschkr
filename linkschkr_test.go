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

	gotFailures, err := links.Check(testURL,
		links.WithNoRecursion(true),
		links.WithStdout(io.Discard),
	)
	if err != nil {
		t.Fatal(err)
	}
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

	gotFailures, err := links.Check(ts.URL,
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
		links.WithIntervalInMs(500),
	)
	if err != nil {
		t.Fatal(err)
	}
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

	gotFailures, err := links.Check(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
		links.WithIntervalInMs(500),
	)
	if err != nil {
		t.Fatal(err)
	}
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
}
