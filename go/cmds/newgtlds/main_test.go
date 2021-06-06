package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/publicsuffix/list/go/datasource/icann"
)

func TestRenderData(t *testing.T) {
	entries := []*icann.GTLDEntry{
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
		},
	}

	expectedList := `// ceepeeyou : 2099-06-13 @cpu's bargain gTLD emporium
ceepeeyou

// cpu : 2019-06-13
ｃｐｕ

`

	var buf bytes.Buffer
	if err := renderGTLDData(io.Writer(&buf), entries); err != nil {
		t.Fatalf("unexpected error from renderData: %v", err)
	}

	if rendered := buf.String(); rendered != expectedList {
		t.Errorf("expected rendered list content %q, got %q",
			expectedList, rendered)
	}
}

func TestErrInvertedSpan(t *testing.T) {
	err := errInvertedSpan{gTLDDatSpan{startIndex: 50, endIndex: 10}}
	expected := `found footer line "// ===END ICANN DOMAINS===" ` +
		`before header line "// newGTLDs" (index 10 vs 50)`
	if actual := err.Error(); actual != expected {
		t.Errorf("expected %#v Error() to return %q got %q", err, expected, actual)
	}
}

func TestGTLDDatSpanValidate(t *testing.T) {
	testCases := []struct {
		name     string
		span     gTLDDatSpan
		expected error
	}{
		{
			name:     "no header",
			span:     gTLDDatSpan{},
			expected: errNoHeader,
		},
		{
			name:     "no footer",
			span:     gTLDDatSpan{startIndex: 10},
			expected: errNoFooter,
		},
		{
			name:     "inverted",
			span:     gTLDDatSpan{startIndex: 50, endIndex: 10},
			expected: errInvertedSpan{gTLDDatSpan{startIndex: 50, endIndex: 10}},
		},
		{
			name: "valid",
			span: gTLDDatSpan{startIndex: 10, endIndex: 20},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := tc.span.validate(); actual != tc.expected {
				t.Errorf("expected span %v validate to return %v got %v",
					tc.span, tc.expected, actual)
			}
		})
	}
}

func TestErrSpanOutOfBounds(t *testing.T) {
	err := errSpanOutOfBounds{
		span:     gTLDDatSpan{startIndex: 5, endIndex: 50},
		numLines: 20,
	}
	expected := `span out of bounds: start index 5, end index 50, number of lines 20`
	if actual := err.Error(); actual != expected {
		t.Errorf("expected %#v Error() to return %q got %q", err, expected, actual)
	}
}

func TestDatFileValidate(t *testing.T) {
	testCases := []struct {
		name     string
		file     datFile
		expected error
	}{
		{
			name:     "bad gTLD span",
			file:     datFile{gTLDSpan: gTLDDatSpan{}},
			expected: errNoHeader,
		},
		{
			name: "out of bounds span",
			file: datFile{
				lines:    []string{"one line"},
				gTLDSpan: gTLDDatSpan{startIndex: 5, endIndex: 10},
			},
			expected: errSpanOutOfBounds{
				span:     gTLDDatSpan{startIndex: 5, endIndex: 10},
				numLines: 1,
			},
		},
		{
			name: "valid",
			file: datFile{
				lines:    []string{"one line", "two line", "three line", "four"},
				gTLDSpan: gTLDDatSpan{startIndex: 2, endIndex: 3}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := tc.file.validate(); actual != tc.expected {
				t.Errorf("expected dat file %v validate to return %v got %v",
					tc.file, tc.expected, actual)
			}
		})
	}
}

func TestGetGTLDLines(t *testing.T) {
	lines := []string{
		"some junk",              // Index 0
		PSL_GTLDS_SECTION_HEADER, // Index 1
		"here be gTLDs",          // Index 2
		"so many gTLDs",          // Index 3
		PSL_GTLDS_SECTION_FOOTER, // Index 4
		"more junk",              // Index 5
	}
	file := datFile{
		lines:    lines,
		gTLDSpan: gTLDDatSpan{startIndex: 2, endIndex: 4},
	}

	expectedLines := []string{
		lines[2], lines[3],
	}

	if actual, err := file.getGTLDLines(); err != nil {
		t.Errorf("unexpected err: %v", err)
	} else if !reflect.DeepEqual(actual, expectedLines) {
		t.Errorf("expected %v got %v", expectedLines, actual)
	}

	// Now update the gTLDSpan to be invalid and try again
	file.gTLDSpan.endIndex = 99
	expectedErr := errSpanOutOfBounds{
		numLines: len(lines),
		span:     gTLDDatSpan{startIndex: 2, endIndex: 99},
	}
	if _, err := file.getGTLDLines(); err != expectedErr {
		t.Errorf("expected err %v got %v", expectedErr, err)
	}
}

func TestReplaceGTLDContent(t *testing.T) {
	origLines := []string{
		"some junk",              // Index 0
		PSL_GTLDS_SECTION_HEADER, // Index 1
		"here be gTLDs",          // Index 2
		"so many gTLDs",          // Index 3
		PSL_GTLDS_SECTION_FOOTER, // Index 4
		"more junk",              // Index 5
	}
	file := datFile{
		lines:    origLines,
		gTLDSpan: gTLDDatSpan{startIndex: 2, endIndex: 4},
	}
	newLines := []string{
		"new gTLD A", // Index 0
		"new gTLD B", // Index 1
		"new gTLD C", // Index 2
	}

	newContent := strings.Join(newLines, "\n")
	if err := file.ReplaceGTLDContent(newContent); err != nil {
		t.Errorf("unexpected err %v", err)
	}

	expectedLines := []string{
		origLines[0],
		origLines[1],
		newLines[0],
		newLines[1],
		newLines[2],
		origLines[4],
		origLines[5],
	}
	if !reflect.DeepEqual(file.lines, expectedLines) {
		t.Errorf("expected lines to be updated to %v was %v", expectedLines, file.lines)
	}
	if file.gTLDSpan.endIndex != 5 {
		t.Errorf("expected file to have gTLDSpan end updated to 5, was %d",
			file.gTLDSpan.endIndex)
	}

	// Now update the gTLDSpan to be invalid and try again
	file.gTLDSpan.endIndex = 99
	expectedErr := errSpanOutOfBounds{
		numLines: len(expectedLines),
		span:     gTLDDatSpan{startIndex: 2, endIndex: 99},
	}
	if err := file.ReplaceGTLDContent("ignored content"); err != expectedErr {
		t.Errorf("expected err %v got %v", expectedErr, err)
	} else if !reflect.DeepEqual(file.lines, expectedLines) {
		t.Errorf("expected lines to still be %v was changed to %v",
			expectedLines, file.lines)
	}
}

func TestDatFileString(t *testing.T) {
	file := datFile{
		lines: []string{"hello", "world"},
	}
	expected := "hello\nworld"
	if actual := file.String(); actual != expected {
		t.Errorf("expected file %v String() to be %q was %q", file, expected, actual)
	}
}

func TestReadDatFile(t *testing.T) {
	mustWriteTemp := func(t *testing.T, content string) string {
		tmpfile, err := ioutil.TempFile("", "dat")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		if _, err := tmpfile.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
		if err := tmpfile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}
		return tmpfile.Name()
	}

	noHeaderContent := strings.Join([]string{
		"foo",
		"bar",
	}, "\n")
	noHeaderFile := mustWriteTemp(t, noHeaderContent)
	defer os.Remove(noHeaderFile)

	noFooterContent := strings.Join([]string{
		"foo",
		PSL_GTLDS_SECTION_HEADER,
		"bar",
	}, "\n")
	noFooterFile := mustWriteTemp(t, noFooterContent)
	defer os.Remove(noFooterFile)

	multiHeaderContent := strings.Join([]string{
		"foo",
		PSL_GTLDS_SECTION_HEADER,
		"test",
		PSL_GTLDS_SECTION_HEADER,
		"test",
		PSL_GTLDS_SECTION_FOOTER,
		"bar",
	}, "\n")
	multiHeaderFile := mustWriteTemp(t, multiHeaderContent)
	defer os.Remove(multiHeaderFile)

	invertedContent := strings.Join([]string{
		"foo",
		PSL_GTLDS_SECTION_FOOTER,
		"test",
		PSL_GTLDS_SECTION_HEADER,
		"bar",
	}, "\n")
	invertedFile := mustWriteTemp(t, invertedContent)
	defer os.Remove(invertedFile)

	validContent := strings.Join([]string{
		"foo",                    // Index 0
		PSL_GTLDS_SECTION_HEADER, // Index 1
		"test",                   // Index 2
		PSL_GTLDS_SECTION_FOOTER, // Index 3
		"bar",                    // Index 4
	}, "\n")
	validFile := mustWriteTemp(t, validContent)
	defer os.Remove(validFile)

	testCases := []struct {
		name            string
		path            string
		expectedErrMsg  string
		expectedDatFile *datFile
	}{
		{
			name:           "no such file",
			path:           "",
			expectedErrMsg: "open : no such file or directory",
		},
		{
			name:           "no header",
			path:           noHeaderFile,
			expectedErrMsg: errNoHeader.Error(),
		},
		{
			name:           "no footer",
			path:           noFooterFile,
			expectedErrMsg: errNoFooter.Error(),
		},
		{
			name:           "multiple headers",
			path:           multiHeaderFile,
			expectedErrMsg: errMultipleHeaders.Error(),
		},
		{
			name:           "inverted header/footer",
			path:           invertedFile,
			expectedErrMsg: (errInvertedSpan{gTLDDatSpan{startIndex: 4, endIndex: 1}}).Error(),
		},
		{
			name: "valid",
			path: validFile,
			expectedDatFile: &datFile{
				lines: strings.Split(validContent, "\n"),
				gTLDSpan: gTLDDatSpan{
					startIndex: 2,
					endIndex:   3,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := readDatFile(tc.path)
			if err != nil && tc.expectedErrMsg == "" {
				t.Errorf("unexpected err: %v", err)
			} else if err != nil && err.Error() != tc.expectedErrMsg {
				t.Errorf("expected err: %q, got: %q", tc.expectedErrMsg, err.Error())
			} else if err == nil && tc.expectedErrMsg != "" {
				t.Errorf("expected err: %q, got: nil", tc.expectedErrMsg)
			} else if !reflect.DeepEqual(actual, tc.expectedDatFile) {
				t.Errorf("expected dat file: %q, got %q", tc.expectedDatFile, actual)
			}
		})
	}
}

type mockClock struct {
	fakeUnixTime int64
}

func (m mockClock) Now() time.Time {
	return time.Unix(m.fakeUnixTime, 0)
}

func TestProcess(t *testing.T) {
	mockHandler := func(content string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, content)
		}
	}

	existingData := `
...

// newGTLDs

// List of new gTLDs imported from https://www.icann.org/resources/registries/gtlds/v2/gtlds.json on 2021-02-07T13:25:56-05:00
// This list is auto-generated, don't edit it manually.
// aaa : 2015-02-26 American Automobile Association, Inc.
aaa


// ===END ICANN DOMAINS===

...
`
	existingJSON := `
{
	"gTLDs": [
		{
			"contractTerminated": false,
			"dateOfContractSignature": "2015-02-26",
			"gTLD": "aaa",
			"registryOperator": "American Automobile Association, Inc.",
			"removalDate": null,
			"uLabel": null
		}
	]
}
`

	newJSON := `
{
	"gTLDs": [
		{
			"contractTerminated": false,
			"dateOfContractSignature": "2015-02-26",
			"gTLD": "aaa",
			"registryOperator": "American Automobile Association, Inc.",
			"removalDate": null,
			"uLabel": null
		},
		{
			"contractTerminated": false,
			"dateOfContractSignature": "2014-03-20",
			"gTLD": "accountants",
			"registryOperator": "Binky Moon, LLC",
			"removalDate": null,
			"uLabel": null
		}
	]
}
`

	fakeClock := mockClock{
		fakeUnixTime: 1612916654,
	}
	newData := `
...

// newGTLDs

// List of new gTLDs imported from https://www.icann.org/resources/registries/gtlds/v2/gtlds.json on 2021-02-10T00:24:14Z
// This list is auto-generated, don't edit it manually.
// aaa : 2015-02-26 American Automobile Association, Inc.
aaa

// accountants : 2014-03-20 Binky Moon, LLC
accountants


// ===END ICANN DOMAINS===

...
`

	mustReadDatFile := func(t *testing.T, content string) *datFile {
		datFile, err := readDatFileContent(content)
		if err != nil {
			t.Fatalf("failed to readDatFileContent %q: %v", content, err)
		}
		return datFile
	}

	testCases := []struct {
		name            string
		file            *datFile
		pslJSON         string
		expectedErrMsg  string
		expectedContent string
	}{
		{
			name:           "bad span",
			file:           &datFile{},
			expectedErrMsg: errNoHeader.Error(),
		},
		{
			name: "span too small",
			file: &datFile{
				lines:    []string{"a", "b", "c"},
				gTLDSpan: gTLDDatSpan{startIndex: 1, endIndex: 2},
			},
			expectedErrMsg: "gtld span data was too small, missing header?",
		},
		{
			name:            "no change in data",
			file:            mustReadDatFile(t, existingData),
			pslJSON:         existingJSON,
			expectedContent: existingData,
		},
		{
			name:            "change in data",
			file:            mustReadDatFile(t, existingData),
			pslJSON:         newJSON,
			expectedContent: newData,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(mockHandler(tc.pslJSON))
			defer s.Close()

			content, err := processGTLDs(tc.file, s.URL, fakeClock)
			if err != nil && tc.expectedErrMsg == "" {
				t.Errorf("unexpected err: %v", err)
			} else if err != nil && err.Error() != tc.expectedErrMsg {
				t.Errorf("expected err: %q, got: %q", tc.expectedErrMsg, err.Error())
			} else if err == nil && tc.expectedErrMsg != "" {
				t.Errorf("expected err: %q, got: nil", tc.expectedErrMsg)
			} else if content != tc.expectedContent {
				fmt.Printf("got content:\n%s", content)
				fmt.Printf("expected content:\n%s", tc.expectedContent)
				t.Errorf("expected content: %q, got %q", tc.expectedContent, content)
			}
		})
	}
}
