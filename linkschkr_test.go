package linkschkr

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCrawler(t *testing.T) {
	t.Parallel()

	nWorkers := 1000 // just for fun
	work := make(chan *Work)
	results := make(chan *Result)

	for n := 0; n < nWorkers; n++ {
		go Crawler(work)
	}

	body := `<html>
	<head>
		<title>For the love of Go!</title>
	</head>
	<body>
		Hi folks,

		See docs <a href="/docs">Go docs</a>. Go is awesome!
		ajkwerjakwerjkaewrhjakwehrjkawe
		aelwrkhakelrhalkewrhkleawlhrwaklerhalkewrhalewrk

		Visit <a href="/live">our live</a>.
		aewrjaewporawe

		awerjaioewrhaewoi
		a href="testintentional.com">trying to get you in trouble</a>
	</body>
</html>`

	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				w.Header().Add("Content-Type", "application/text")
				w.Write([]byte(body))
			}
		}),
	)
	defer ts.Close()

	work <- &Work{
		site:   ts.URL,
		result: results,
	}

	result := <-results
	wantUp := true
	gotUp := result.up
	if wantUp != gotUp {
		t.Errorf("want site up %t, got %t", wantUp, gotUp)
	}

	wantURIs := []string{"/docs", "/live"}
	gotURIs := result.extraURIs

	if !cmp.Equal(wantURIs, gotURIs) {
		t.Errorf(cmp.Diff(wantURIs, gotURIs))
	}
}

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

	// wantURIs := []string{}
	// result := <-checker.Result
	// gotURIs := result.extraURIs

	// if !cmp.Equal(wantURIs, gotURIs) {
	// 	t.Errorf(cmp.Diff(wantURIs, gotURIs))
	// }
}
