package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"
)

const (
	// ICANN_GTLD_JSON_URL is the URL for the ICANN gTLD JSON registry (version
	// 2). See https://www.icann.org/resources/pages/registries/registries-en for
	// more information.
	ICANN_GTLD_JSON_URL = "https://www.icann.org/resources/registries/gtlds/v2/gtlds.json"
	// IANA_TLDS_TXT_URL is the URL for the IANA "Public Suffix List" of TLDs
	// in the ICP-3 Root - including new ccTLDs, EBRERO gTLDS or things not in
	// the JSON File above that should be included in the PSL.  Note: UPPERCASE
	IANA_TLDS_TXT_URL = "http://data.iana.org/TLD/tlds-alpha-by-domain.txt"
	// PSL_GTLDS_SECTION_HEADER marks the start of the newGTLDs section of the
	// overall public suffix dat file.
	PSL_GTLDS_SECTION_HEADER = "// newGTLDs"
	// PSL_GTLDS_SECTION_FOOTER marks the end of the newGTLDs section of the
	// overall public suffix dat file.
	PSL_GTLDS_SECTION_FOOTER = "// ===END ICANN DOMAINS==="
)

var (
	// legacyGTLDs are gTLDs that predate ICANN's new gTLD program. These legacy
	// gTLDs are present in the ICANN_GTLD_JSON_URL data but we do not want to
	// include them in the new gTLD section of the PSL data because it will create
	// duplicates with existing entries alongside registry-reserved second level
	// domains present in the PSL data. Entries present in legacyGTLDs will not be
	// output by this tool when generating the new gTLD data.
	legacyGTLDs = map[string]bool{
		"aero":   true,
		"asia":   true,
		"biz":    true,
		"cat":    true,
		"com":    true,
		"coop":   true,
		"info":   true,
		"jobs":   true,
		"mobi":   true,
		"museum": true,
		"name":   true,
		"net":    true,
		"org":    true,
		"post":   true,
		"pro":    true,
		"tel":    true,
		"xxx":    true,
	}

	// pslGTLDHeaderTemplate is a parsed text/template instance for rendering the
	// header before the data rendered with the pslGTLDTemplate. We use two separate
	// templates so that we can avoid having a variable date stamp in the
	// pslGTLDTemplate, allowing us to easily check that the data in the current
	// .dat file is unchanged from what we render when there are no updates to
	// add.
	//
	// Expected template data:
	//   URL - the string URL that the data was fetched from.
	//   Date - the time.Date that the data was fetched.
	//   DateFormat - the format string to use with the date.
	pslGTLDHeaderTemplate = template.Must(
		template.New("public-suffix-list-gtlds-header").Parse(`
// List of new gTLDs imported from {{ .URL }} on {{ .Date.Format .DateFormat }}
// This list is auto-generated, don't edit it manually.`))

	// pslGTLDTemplate is a parsed text/template instance for rendering a list of
	// gtldEntry objects in the format used by the public suffix list.
	//
	// It expects the following template data:
	//   Entries - a list of gtldEntry objects.
	pslGTLDTemplate = template.Must(
		template.New("public-suffix-list-gtlds").Parse(`
{{- range .Entries }}
{{- .Comment }}
{{ printf "%s\n" .ULabel }}
{{ end }}`))
)

// gtldEntry is a struct matching a subset of the gTLD data fields present in
// each object entry of the "GLTDs" array from ICANN_GTLD_JSON_URL.
type gtldEntry struct {
	// ALabel contains the ASCII gTLD name. For internationalized gTLDs the GTLD
	// field is expressed in punycode.
	ALabel string `json:"gTLD"`
	// ULabel contains the unicode representation of the gTLD name. When the gTLD
	// ULabel in the ICANN gTLD data is empty (e.g for an ASCII gTLD like
	// '.pizza') the PSL entry will use the ALabel as the ULabel.
	ULabel string
	// RegistryOperator holds the name of the registry operator that operates the
	// gTLD (may be empty).
	RegistryOperator string
	// DateOfContractSignature holds the date the gTLD contract was signed (may be empty).
	DateOfContractSignature string
	// ContractTerminated indicates whether the contract has been terminated by
	// ICANN. When rendered by the pslGTLDTemplate only entries with
	// ContractTerminated = false are included.
	ContractTerminated bool
	// RemovalDate indicates the date the gTLD delegation was removed from the
	// root zones.
	RemovalDate string
}

// normalize will normalize a gtldEntry by mutating it in place to trim the
// string fields of whitespace and by populating the ULabel with the ALabel if
// the ULabel is empty.
func (e *gtldEntry) normalize() {
	e.ALabel = strings.TrimSpace(e.ALabel)
	e.ULabel = strings.TrimSpace(e.ULabel)
	e.RegistryOperator = strings.TrimSpace(e.RegistryOperator)
	e.DateOfContractSignature = strings.TrimSpace(e.DateOfContractSignature)

	// If there is no explicit uLabel use the gTLD as the uLabel.
	if e.ULabel == "" {
		e.ULabel = e.ALabel
	}
}

// Comment generates a comment string for the gtldEntry. This string has a `//`
// prefix and matches one of the following two forms.
//
// If the registry operator field is empty the comment will be of the form:
//
//    '// <ALabel> : <DateOfContractSignature>'
//
// If the registry operator field is not empty the comment will be of the form:
//
//    '// <ALabel> : <DateOfContractSignature> <RegistryOperator>'
//
// In both cases the <DateOfContractSignature> may be empty.
func (e gtldEntry) Comment() string {
	parts := []string{
		"//",
		e.ALabel,
		":",
		e.DateOfContractSignature,
	}
	// Avoid two trailing spaces if registry operator is empty
	if e.RegistryOperator != "" {
		parts = append(parts, e.RegistryOperator)
	}
	return strings.Join(parts, " ")
}

// filterGTLDs removes entries that are present in the legacyGTLDs map or have
// ContractTerminated equal to true, or a non-empty RemovalDate.
func filterGTLDs(entries []*gtldEntry) []*gtldEntry {
	var filtered []*gtldEntry
	for _, entry := range entries {
		if _, isLegacy := legacyGTLDs[entry.ALabel]; isLegacy {
			continue
		}
		if entry.ContractTerminated {
			continue
		}
		if entry.RemovalDate != "" {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// getGTLDPSLEntries fetches a list of gtldEntry objects (or returns an error) by:
//   1. getting the raw JSON data from the provided url string.
//   2. unmarshaling the JSON data to create gtldEntry objects.
//   3. normalizing the gtldEntry objects.
//   4. filtering out any legacy or contract terminated gTLDs
//
// If there are no gtldEntry objects after unmarshaling the data in step 2 or
// filtering the gTLDs in step 4 it is considered an error condition.
func getGTLDPSLEntries(url string) ([]*gtldEntry, error) {
	respBody, err := getData(url)
	if err != nil {
		return nil, err
	}

	var results struct {
		GTLDs []*gtldEntry
	}
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf(
			"unmarshaling ICANN gTLD JSON data: %v", err)
	}

	// We expect there to always be GTLD data. If there was none after unmarshaling
	// then its likely the data format has changed or something else has gone wrong.
	if len(results.GTLDs) == 0 {
		return nil, errors.New("found no gTLD information after unmarshaling")
	}

	// Normalize each tldEntry. This will remove leading/trailing whitespace and
	// populate the ULabel with the ALabel if the entry has no ULabel.
	for _, tldEntry := range results.GTLDs {
		tldEntry.normalize()
	}

	filtered := filterGTLDs(results.GTLDs)
	if len(filtered) == 0 {
		return nil, errors.New(
			"found no gTLD information after removing legacy and contract terminated gTLDs")
	}
	return filtered, nil
}

// renderGTLDHeader renders the pslGTLDHeaderTemplate to the writer or returns an
// error. The provided clock instance is used for the header last update
// timestamp. If no clk instance is provided realClock is used.
func renderGTLDHeader(writer io.Writer, clk clock) error {
	if clk == nil {
		clk = &realClock{}
	}
	templateData := struct {
		URL        string
		Date       time.Time
		DateFormat string
	}{
		URL:        ICANN_GTLD_JSON_URL,
		Date:       clk.Now().UTC(),
		DateFormat: time.RFC3339,
	}

	return renderTemplate(writer, pslGTLDHeaderTemplate, templateData)
}

// renderGTLDData renders the given list of gtldEntry objects using the
// pslGTLDTemplate. The rendered template data is written to the provided writer
// or an error is returned.
func renderGTLDData(writer io.Writer, entries []*gtldEntry) error {
	templateData := struct {
		Entries []*gtldEntry
	}{
		Entries: entries,
	}

	return renderTemplate(writer, pslGTLDTemplate, templateData)
}

// Process handles updating a datFile with new gTLD content. If there are no
// gTLD updates the existing dat file's contents will be returned. If there are
// updates, the new updates will be spliced into place and the updated file contents
// returned.
func processGTLDs(datFile *datFile, dataURL string, clk clock) (string, error) {
	// Get the lines for the gTLD data span - this includes both the header with the
	// date and the actual gTLD entries.
	spanLines, err := datFile.getGTLDLines()
	if err != nil {
		return "", err
	}

	// Render a new header for the gTLD data.
	var newHeaderBuf strings.Builder
	if err := renderGTLDHeader(&newHeaderBuf, clk); err != nil {
		return "", err
	}

	// Figure out how many lines the header with the dynamic date is.
	newHeaderLines := strings.Split(newHeaderBuf.String(), "\n")
	headerLen := len(newHeaderLines)

	// We should have at least that many lines in the existing span data.
	if len(spanLines) <= headerLen {
		return "", errors.New("gtld span data was too small, missing header?")
	}

	// The gTLD data can be found by skipping the header lines
	existingData := strings.Join(spanLines[headerLen:], "\n")

	// Fetch new gTLD PSL entries.
	entries, err := getGTLDPSLEntries(dataURL)
	if err != nil {
		return "", err
	}

	// Render the new gTLD PSL section with the new entries.
	var newDataBuf strings.Builder
	if err := renderGTLDData(&newDataBuf, entries); err != nil {
		return "", err
	}

	// If the newly rendered data doesn't match the existing data then we want to
	// update the dat file content by replacing the old span with the new content.
	if newDataBuf.String() != existingData {
		newContent := newHeaderBuf.String() + "\n" + newDataBuf.String()
		if err := datFile.ReplaceGTLDContent(newContent); err != nil {
			return "", err
		}
	}

	return datFile.String(), nil
}
