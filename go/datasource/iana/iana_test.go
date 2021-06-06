package iana

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestTLDEntryNormalize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		entry    TLDEntry
		expected TLDEntry
	}{
		{
			name:     "lowercase",
			entry:    TLDEntry("cpu"),
			expected: TLDEntry("cpu"),
		},
		{
			name:     "uppercase",
			entry:    TLDEntry("CPU"),
			expected: TLDEntry("cpu"),
		},
		{
			name:     "mixedcase",
			entry:    TLDEntry("cPu"),
			expected: TLDEntry("cpu"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.entry.Normalize(); tc.entry != tc.expected {
				t.Errorf("expected %q got %q", tc.expected, tc.entry)
			}
		})
	}
}

type mockHandler struct {
	respData []byte
}

func (h *mockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write(h.respData)
}

func TestGetTLDs(t *testing.T) {
	t.Parallel()

	mockData := `
# Mock TLD data

CPU
CEEPEEYOU
XN--UNUP4Y
`

	expectedEntries := []TLDEntry{"cpu", "ceepeeyou", "xn--unup4y"}

	handler := &mockHandler{[]byte(mockData)}
	server := httptest.NewServer(handler)
	defer server.Close()

	entries, err := GetTLDs(server.URL)
	if err != nil {
		t.Fatalf("expected no error from GetTLDs with mockHandler. Got %v",
			err)
	}

	for i, entry := range entries {
		if deepEqual := reflect.DeepEqual(*entry, expectedEntries[i]); !deepEqual {
			t.Errorf("getTLDPSLEntries() entry index %d was %#v, expected %#v",
				i,
				*entry,
				expectedEntries[i])
		}
	}
}

func TestGetTLDsEmptyResults(t *testing.T) {
	t.Parallel()

	// Mock an empty result
	handler := &mockHandler{[]byte{}}
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := GetTLDs(server.URL)
	if err == nil {
		t.Error("expected error from getGTLDPSLEntries with empty results mockHandler. Got nil")
	}
}
