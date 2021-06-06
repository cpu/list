package icann

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestEntryNormalize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		inputEntry    GTLDEntry
		expectedEntry GTLDEntry
	}{
		{
			name: "already normalized",
			inputEntry: GTLDEntry{
				ALabel:                  "cpu",
				ULabel:                  "ｃｐｕ",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
			expectedEntry: GTLDEntry{
				ALabel:                  "cpu",
				ULabel:                  "ｃｐｕ",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
		},
		{
			name: "extra whitespace",
			inputEntry: GTLDEntry{
				ALabel:                  "  cpu    ",
				ULabel:                  "   ｃｐｕ   ",
				DateOfContractSignature: "   2019-06-13    ",
				RegistryOperator: "     @cpu's bargain gTLD emporium " +
					"(now with bonus whitespace)    ",
			},
			expectedEntry: GTLDEntry{
				ALabel:                  "cpu",
				ULabel:                  "ｃｐｕ",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator: "@cpu's bargain gTLD emporium " +
					"(now with bonus whitespace)",
			},
		},
		{
			name: "no explicit uLabel",
			inputEntry: GTLDEntry{
				ALabel:                  "cpu",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
			expectedEntry: GTLDEntry{
				ALabel:                  "cpu",
				ULabel:                  "cpu",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry := &tc.inputEntry
			entry.Normalize()
			if deepEqual := reflect.DeepEqual(*entry, tc.expectedEntry); !deepEqual {
				t.Errorf("entry did not match expected after normalization. %v vs %v",
					*entry, tc.expectedEntry)
			}
		})
	}
}

func TestEntryComment(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		entry    GTLDEntry
		expected string
	}{
		{
			name: "Full entry",
			entry: GTLDEntry{
				ALabel:                  "cpu",
				DateOfContractSignature: "2019-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
			expected: "// cpu : 2019-06-13 @cpu's bargain gTLD emporium",
		},
		{
			name: "Entry with empty contract signature date and operator",
			entry: GTLDEntry{
				ALabel: "cpu",
			},
			expected: "// cpu : ",
		},
		{
			name: "Entry with empty contract signature and non-empty operator",
			entry: GTLDEntry{
				ALabel:           "cpu",
				RegistryOperator: "@cpu's bargain gTLD emporium",
			},
			expected: "// cpu :  @cpu's bargain gTLD emporium",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if actual := tc.entry.Comment(); actual != tc.expected {
				t.Errorf("entry %v Comment() == %q expected == %q",
					tc.entry, actual, tc.expected)
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

func TestGetGTLDs(t *testing.T) {
	mockData := struct {
		GTLDs []GTLDEntry
	}{
		GTLDs: []GTLDEntry{
			{
				ALabel:                  "ceepeeyou",
				DateOfContractSignature: "2099-06-13",
				RegistryOperator:        "@cpu's bargain gTLD emporium",
			},
			{
				// NOTE: we include whitespace in this entry to test that normalization
				// occurs.
				ALabel:                  "  cpu    ",
				ULabel:                  "   ｃｐｕ   ",
				DateOfContractSignature: "   2019-06-13    ",
				RegistryOperator: "     @cpu's bargain gTLD emporium " +
					"(now with bonus whitespace)    ",
			},
			{
				// NOTE: we include a legacy gTLD here to test that filtering of legacy
				// gTLDs occurs.
				ALabel:                  "aero",
				DateOfContractSignature: "1999-10-31",
				RegistryOperator:        "Department of Historical Baggage and Technical Debt",
			},
			{
				ALabel:                  "terminated-not-yet-delegated",
				DateOfContractSignature: "1987-10-31",
				// NOTE: we include a contract terminated = true entry here, with no date of
				// delegation to test that filtering of terminated and undelegated
				// entries occurs.
				ContractTerminated: true,
			},
			{
				ALabel:                  "terminated-and-delegated",
				DateOfContractSignature: "1987-10-31",
				// NOTE: we include a contract terminated = true entry here with a date
				// of delegation to ensure that it remains post-filtering.
				DateOfDelegation:   "2021-06-05",
				ContractTerminated: true,
			},
			{
				ALabel:                  "terminated-and-removed",
				DateOfContractSignature: "1987-10-31",
				// NOTE: we include a contract terminated = true entry here with a date
				// of delegation *and* a removal date to ensure that filtering of removed
				// entries occurs.
				DateOfDelegation:   "2021-06-05",
				ContractTerminated: true,
				RemovalDate:        "2021-06-06",
			},
		},
	}
	// NOTE: swallowing the possible err return here because the mock data is
	// assumed to be static/correct and it simplifies the handler.
	jsonBytes, _ := json.Marshal(mockData)

	expectedEntries := []GTLDEntry{
		{
			ALabel:                  "ceepeeyou",
			ULabel:                  "ceepeeyou",
			DateOfContractSignature: "2099-06-13",
			RegistryOperator:        "@cpu's bargain gTLD emporium",
		},
		{
			ALabel:                  "cpu",
			ULabel:                  "ｃｐｕ",
			DateOfContractSignature: "2019-06-13",
			RegistryOperator: "@cpu's bargain gTLD emporium " +
				"(now with bonus whitespace)",
		},
		{
			ALabel:                  "terminated-and-delegated",
			ULabel:                  "terminated-and-delegated",
			DateOfContractSignature: "1987-10-31",
			DateOfDelegation:        "2021-06-05",
			ContractTerminated:      true,
		},
	}

	handler := &mockHandler{jsonBytes}
	server := httptest.NewServer(handler)
	defer server.Close()

	entries, err := GetGTLDs(server.URL)
	if err != nil {
		t.Fatalf("expected no error from getGTLDPSLEntries with mockHandler. Got %v",
			err)
	}

	if len(entries) != len(expectedEntries) {
		t.Fatalf("expected %d entries from getGTLDPSLEntries with mockHandler. Got %d",
			len(expectedEntries),
			len(entries))
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

func TestGetGTLDsEmptyResults(t *testing.T) {
	t.Parallel()

	// Mock an empty result
	mockData := struct {
		GTLDs []GTLDEntry
	}{}

	// NOTE: swallowing the possible err return here because the mock data is
	// assumed to be static/correct and it simplifies the handler.
	jsonBytes, _ := json.Marshal(mockData)

	handler := &mockHandler{jsonBytes}
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := GetGTLDs(server.URL)
	if err == nil {
		t.Error("expected error from getGTLDPSLEntries with empty results mockHandler. Got nil")
	}
}

func TestGetGTLDsEmptyFilteredResults(t *testing.T) {
	t.Parallel()

	// Mock data that will be filtered to an empty list
	mockData := struct {
		GTLDs []GTLDEntry
	}{
		GTLDs: []GTLDEntry{
			{
				// NOTE: GTLD matches a legacyGTLDs map entry to ensure filtering.
				ALabel:                  "aero",
				DateOfContractSignature: "1999-10-31",
				RegistryOperator:        "Department of Historical Baggage and Technical Debt",
			},
			{
				ALabel:                  "terminated",
				DateOfContractSignature: "1987-10-31",
				// NOTE: Setting ContractTerminated and no DateOfDelegation to ensure
				// filtering.
				ContractTerminated: true,
			},
			{
				ALabel:                  "removed",
				DateOfContractSignature: "1999-10-31",
				RegistryOperator:        "Department of Historical Baggage and Technical Debt",
				// NOTE: Setting RemovalDate to ensure filtering.
				RemovalDate: "2019-08-06",
			},
		},
	}

	// NOTE: swallowing the possible err return here because the mock data is
	// assumed to be static/correct and it simplifies the handler.
	jsonBytes, _ := json.Marshal(mockData)

	handler := &mockHandler{jsonBytes}
	server := httptest.NewServer(handler)
	defer server.Close()

	_, err := GetGTLDs(server.URL)
	if err == nil {
		t.Error("expected error from getGTLDPSLEntries with empty filtered results mockHandler. Got nil")
	}
}
