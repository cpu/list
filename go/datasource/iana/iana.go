package iana

import (
	"errors"
	"strings"

	"github.com/publicsuffix/list/go/datasource"
)

const (
	// TLDS_BY_DOMAIN_TXT_URL is the URL to IANA's list of domains in the ICP-3
	// Root - including new ccTLDs, EBRERO gTLDS and things not in the ICANN GTLD
	// JSON Registry. Note: TLDs within this TXT registry are in UPPERCASE but are
	// converted to lowercase by TLDEntry.normalize().
	TLDS_BY_DOMAIN_TXT_URL = "https://data.iana.org/TLD/tlds-alpha-by-domain.txt"
)

type TLDEntry string

func (t *TLDEntry) Normalize() {
	normed := TLDEntry(strings.ToLower((string)(*t)))
	*t = normed
}

// GetTLDs fetches a list of TLDEntry objects (or returns an
// error) by:
//   1. getting the raw TXT data from the provided url string.
//   2. processing the TXT data to create TLDEntry objects.
//   3. normalizing the TLDEntry objects.
//
// If there are no TLDEntry objects after unmarshaling the data in step 2
// it is considered an error condition.
func GetTLDs(url string) ([]*TLDEntry, error) {
	respBody, err := datasource.GetHTTPData(url)
	if err != nil {
		return nil, err
	}

	var results []*TLDEntry
	lines := strings.Split(string(respBody), "\n")
	for _, line := range lines {
		// Skip comment lines, or empty lines
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		entry := TLDEntry(strings.TrimSpace(line))
		results = append(results, &entry)
	}

	// We expect there to always be TLD data. If there was none after unmarshaling
	// then its likely the data format has changed or something else has gone wrong.
	if len(results) == 0 {
		return nil, errors.New("found no TLD information")
	}

	// Normalize each tldEntry. This will remove leading/trailing whitespace and
	// populate the ULabel with the ALabel if the entry has no ULabel.
	for _, tldEntry := range results {
		tldEntry.Normalize()
	}

	return results, nil
}
