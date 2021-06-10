package links_test

import (
	"fmt"
	"io"
	"links"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestCheckValidLinkIntegration(t *testing.T) {
	t.Parallel()
	testURL := os.Getenv("LINKSCHKR_TESTS_url")
	if testURL == "" {
		t.Skip("Set LINKSCHKR_TESTS_url=<url to check> to run integration tests")
	}

	gotFailures, err := links.Check([]string{testURL},
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

func TestCheckValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))

	gotFailures, err := links.Check([]string{ts.URL},
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
		links.WithIntervalInMs(500),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors()) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors()))
	}
}

func TestCheckNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	gotFailures, err := links.Check([]string{ts.URL},
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
		links.WithIntervalInMs(500),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{
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

func TestCheckBrokenLink(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/href_broken_link.html")
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(content))
	}))
	gotFailures, err := links.Check([]string{ts.URL},
		links.WithHTTPClient(&http.Client{
			Timeout: time.Second,
		}),
		links.WithStdout(io.Discard),
		links.WithIntervalInMs(1),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []links.Result{{
		State: "down",
		URL:   "http://127.0.0.1:0",
	}}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.IgnoreFields(links.Result{}, "Error", "Refer")) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.IgnoreFields(links.Result{}, "Refer")))
	}
}
