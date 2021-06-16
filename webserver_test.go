package links_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"links"
)

func TestCheckWebServer(t *testing.T) {
	t.Parallel()
	request, _ := http.NewRequest(http.MethodGet, "/check?site=https://bitfieldconsulting.com&no-recursion=true", nil)
	response := httptest.NewRecorder()
	links.WebServerHandler(response, request)
	got := response.Body.String()
	if http.StatusOK != response.Code {
		t.Errorf("want response code %d got %d. Body %q", http.StatusOK, response.Code, got)
	}
}
