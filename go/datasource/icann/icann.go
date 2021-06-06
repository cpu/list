package icann

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/publicsuffix/list/go/datasource"
)

const (
	// GTLD_JSON_REGISTRY_URL is the URL for the ICANN gTLD JSON registry (version
	// 2). See https://www.icann.org/resources/pages/registries/registries-en for
	// more information.
	GTLD_JSON_REGISTRY_URL = "https://www.icann.org/resources/registries/gtlds/v2/gtlds.json"
)

var (
	// legacyGTLDs are gTLDs that predate ICANN's new gTLD program. This is a static
	// list that is not expected to change over time.
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
)

// GTLDEntry is a struct matching a subset of the gTLD data fields present in
// each object entry of the "GLTDs" array in the GTLD_JSON_REGISTRY_URL data.
type GTLDEntry struct {
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
	// DateOfDelegation holds the date the gTLD was delegated from a root zone
	// (may be empty).
	DateOfDelegation string
	// ContractTerminated indicates whether the contract has been terminated by
	// ICANN. When rendered by the pslGTLDTemplate only entries with
	// ContractTerminated = false are included.
	ContractTerminated bool
	// RemovalDate indicates the date the gTLD delegation was removed from the
	// root zones.
	RemovalDate string
}

// Normalize will normalize a GTLDEntry by mutating it in place to trim the
// string fields of whitespace and by populating the ULabel with the ALabel if
// the ULabel is empty.
func (e *GTLDEntry) Normalize() {
	e.ALabel = strings.TrimSpace(e.ALabel)
	e.ULabel = strings.TrimSpace(e.ULabel)
	e.RegistryOperator = strings.TrimSpace(e.RegistryOperator)
	e.DateOfContractSignature = strings.TrimSpace(e.DateOfContractSignature)

	// If there is no explicit uLabel use the gTLD as the uLabel.
	if e.ULabel == "" {
		e.ULabel = e.ALabel
	}
}

// Comment generates a comment string for the GTLDEntry. This string has a `//`
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
//
// If the contract has been terminated, but the gTLD has not yet been removed the
// comment will end with the string "(cancelled)".
func (e GTLDEntry) Comment() string {
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
	if e.ContractTerminated {
		parts = append(parts, "(cancelled)")
	}
	return strings.Join(parts, " ")
}

// IsLegacyGTLD returns true if the provided tld (no leading `.`) is a gTLD that
// predates ICANN's modern gTLD program. These legacy gTLDs are present in the
// JSON gTLD registry but we do not want to treat them differently for the purposes
// of the public suffix list data.
func IsLegacyGTLD(tld string) bool {
	return legacyGTLDs[strings.ToLower(tld)]
}

// filterGTLDs removes entries that are present in the legacyGTLDs map, that have
// a ContractTerminated field equal to true and no delegation date, or
// a non-empty RemovalDate. Entries with ContractTerminated true and a delegation
// date (but no removal date) are left in.
func filterGTLDs(entries []*GTLDEntry) []*GTLDEntry {
	var filtered []*GTLDEntry
	for _, entry := range entries {
		if IsLegacyGTLD(entry.ALabel) {
			continue
		}
		if entry.ContractTerminated && entry.DateOfDelegation == "" {
			continue
		}
		if entry.RemovalDate != "" {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// GetGTLDs fetches a list of GTLDEntry objects (or returns an
// error) by:
//   1. getting the raw JSON data from the provided url string.
//   2. unmarshaling the JSON data to create GTLDEntry objects.
//   3. normalizing the GTLDEntry objects.
//   4. filtering out any legacy or contract terminated gTLDs
//
// If there are no GTLDEntry objects after unmarshaling the data in step 2 or
// filtering the gTLDs in step 4 it is considered an error condition.
func GetGTLDs(url string) ([]*GTLDEntry, error) {
	respBody, err := datasource.GetHTTPData(url)
	if err != nil {
		return nil, err
	}

	var results struct {
		GTLDs []*GTLDEntry
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
		tldEntry.Normalize()
	}

	filtered := filterGTLDs(results.GTLDs)
	if len(filtered) == 0 {
		return nil, errors.New(
			"found no gTLD information after removing legacy and contract terminated gTLDs")
	}
	return filtered, nil
}
