package links_test

import (
	"fmt"
	"io"
	"links"
	"log"
	"net"
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

	gotFailures, _, err := links.Check(testURL,
		links.WithRecursive(false),
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

	gotFailures, _, err := links.Check(ts.URL,
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
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

	gotFailures, _, err := links.Check(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantFailures := []*links.Result{{URL: ts.URL, State: "down", ResponseCode: http.StatusNotFound}}
	if !cmp.Equal(wantFailures, gotFailures, cmpopts.EquateErrors(), cmp.AllowUnexported(links.Result{})) {
		t.Errorf(cmp.Diff(wantFailures, gotFailures, cmpopts.EquateErrors(), cmp.AllowUnexported(links.Result{})))
	}
}

func TestExternalLink(t *testing.T) {
	t.Parallel()
	f, err := os.Open("testdata/external_links.html")
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		log.Fatal(err)
	}

	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(content))
	}))
	ts.Listener.Close()
	ts.Listener = l
	ts.Start()
	defer ts.Close()
	gotFailures, gotSuccesses, err := links.Check(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
		links.WithIntervalInMs(500),
		links.WithTimeoutInMs(1000),
	)
	if err != nil {
		t.Fatal(err)
	}
	wantFailLen := 0
	if wantFailLen != len(gotFailures) {
		t.Errorf("want %d items failed got %d", wantFailLen, len(gotFailures))
	}
	wantLen := 3
	if wantLen != len(gotSuccesses) {
		t.Errorf("want %d items succeed got %d", wantLen, len(gotSuccesses))
	}
	wantSuccesses := []*links.Result{
		{ResponseCode: 200, State: "up", URL: "http://127.0.0.1:8080"},
		{ResponseCode: 200, State: "up", URL: "https://golang.org"},
		{ResponseCode: 200, State: "up", URL: "http://www.google.com"},
	}
	if !cmp.Equal(wantSuccesses, gotSuccesses, cmpopts.IgnoreFields(links.Result{}, "refer"), cmp.AllowUnexported(links.Result{})) {
		t.Errorf(cmp.Diff(wantSuccesses, gotSuccesses, cmpopts.IgnoreFields(links.Result{}, "refer"), cmp.AllowUnexported(links.Result{})))
	}

}
