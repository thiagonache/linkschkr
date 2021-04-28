package links_test

import (
	"io"
	"links"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestValidLinkIntegration(t *testing.T) {
	t.Parallel()
	if os.Getenv("LINKSCHKR_TESTS_url") == "" {
		t.Skip("Set LINKSCHKR_TESTS_url=<url to check> to run integration tests")
	}
	links.Run(os.Getenv("LINKSCHKR_TESTS_url"),
		links.WithRunRecursively(false),
		links.WithStdout(io.Discard),
	)
	// gotSuccess, gotFailures, err := checker.Run()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// wantFailures := []links.Result{}
	// if !cmp.Equal(wantFailures, gotFailures) {
	// 	t.Errorf(cmp.Diff(wantFailures, gotFailures))
	// }
	// wantSuccess := []links.Result{
	// 	{
	// 		URL:        "https://golang.org/",
	// 		Err:        nil,
	// 		StatusCode: 200,
	// 		ExtraSites: []string{
	// 			"https://golang.org/",
	// 			"https://golang.org/doc/",
	// 			"https://golang.org/pkg/",
	// 			"https://golang.org/project/",
	// 			"https://golang.org/help/",
	// 			"https://golang.org/blog/",
	// 			"https://golang.org/dl/",
	// 			"https://golang.org/doc/copyright.html",
	// 			"https://golang.org/doc/tos.html",
	// 		},
	// 	},
	// }
	// if !cmp.Equal(wantSuccess, gotSuccess) {
	// 	t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	// }
}

func TestValidLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	links.Run(ts.URL,
		links.WithRunRecursively(false),
		links.WithHTTPClient(ts.Client()),
		links.WithStdout(io.Discard),
	)
	// gotSuccess, gotFailures, err := checker.Run()
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// wantFailures := []links.Result{}
	// if !cmp.Equal(wantFailures, gotFailures) {
	// 	t.Errorf(cmp.Diff(wantFailures, gotFailures))
	// }
	// wantSuccess := []links.Result{
	// 	{
	// 		URL:        ts.URL,
	// 		StatusCode: 200,
	// 		ExtraSites: nil,
	// 	},
	// }
	// if !cmp.Equal(wantSuccess, gotSuccess) {
	// 	t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	// }
}

func TestNotFoundLink(t *testing.T) {
	t.Parallel()
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	links.Run(ts.URL,
		links.WithStdout(io.Discard),
		links.WithHTTPClient(ts.Client()),
	)
	// 	gotSuccess, gotFailures, err := checker.Run()
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	wantFailures := []links.Result{
	// 		{
	// 			URL:        ts.URL,
	// 			Err:        nil,
	// 			StatusCode: http.StatusNotFound,
	// 		},
	// 	}
	// 	if !cmp.Equal(wantFailures, gotFailures) {
	// 		t.Errorf(cmp.Diff(wantFailures, gotFailures))
	// 	}
	// 	wantSuccess := []links.Result{}
	// 	if !cmp.Equal(wantSuccess, gotSuccess) {
	// 		t.Errorf(cmp.Diff(wantSuccess, gotSuccess))
	// 	}
}
