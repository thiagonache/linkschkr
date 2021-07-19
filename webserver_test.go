package links_test

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"links"

	"github.com/google/go-cmp/cmp"
)

func TestCheckWebServer(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequest(http.MethodGet, "/check?site=https://bitfieldconsulting.com&no-recursion=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	ws := links.NewWebServer()
	ws.WebServerHandler(response, request)
	got := response.Body.String()
	if http.StatusOK != response.Code {
		t.Errorf("want response code %d got %d. Body %q", http.StatusOK, response.Code, got)
	}
}

func fakeCheck(sites []string, opts ...links.Option) ([]links.Result, error) {
	return []links.Result{
		{
			ResponseCode: 200,
			State:        "up",
			URL:          "https://bitfieldconsulting.com",
		},
		{
			ResponseCode: 503,
			State:        "down",
			URL:          "https://java.com",
		},
	}, nil
}
func TestCheck(t *testing.T) {
	ws := links.NewWebServer()
	ws.CheckFn = fakeCheck
	go func() {
		err := ws.ListenAndServe()
		if err != nil {
			panic(err)
		}
	}()
	conn, err := net.Dial("tcp", "localhost:8080")
	for err != nil {
		time.Sleep(50 * time.Millisecond)
		conn, err = net.Dial("tcp", "localhost:8080")
	}
	conn.Close()
	resp, err := http.Get("http://localhost:8080/check?site=https://bitfieldconsulting.com")
	if err != nil {
		t.Error(err)
	}
	if http.StatusOK != resp.StatusCode {
		t.Errorf("want response code %d but got %d", http.StatusOK, resp.StatusCode)
	}
	wantBody := "return in a bit"
	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if wantBody != string(gotBody) {
		t.Errorf("want body %q but got %q", wantBody, gotBody)
	}

	resp, err = http.Get("http://localhost:8080/check?site=https://bitfieldconsulting.com")
	if err != nil {
		t.Error(err)
	}
	if http.StatusOK != resp.StatusCode {
		t.Errorf("want response code %d but got %d", http.StatusOK, resp.StatusCode)
	}
	wantBody = `[{"error":null,"refer":"","responseCode":200,"state":"up","url":"https://bitfieldconsulting.com"},{"error":null,"refer":"","responseCode":503,"state":"down","url":"https://java.com"}]`
	gotBody, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(wantBody, string(gotBody)) {
		t.Errorf(cmp.Diff(wantBody, string(gotBody)))
	}
}
