package datasource

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type badStatusHandler struct{}

func (h *badStatusHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusUnavailableForLegalReasons)
	_, _ = w.Write([]byte("sorry"))
}

func TestGetData(t *testing.T) {
	handler := &badStatusHandler{}
	server := httptest.NewServer(handler)
	defer server.Close()

	// NOTE: TestGetData only tests the handling of non-200 status codes in
	// getHTTPData as anything else is just testing stdlib code.
	resp, err := GetHTTPData(server.URL)
	if err == nil {
		t.Error("expected GetHTTPData() to a bad status handler server to return an " +
			"error, got nil")
	}
	if resp != nil {
		t.Errorf("expected GetHTTPData() to a bad status handler server to return a "+
			"nil response body byte slice, got: %v",
			resp)
	}
}
