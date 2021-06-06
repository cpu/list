package main

import (
	"errors"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/publicsuffix/list/go/datasource/icann"
)

const (
	// PSL_GTLDS_SECTION_HEADER marks the start of the newGTLDs section of the
	// overall public suffix dat file.
	PSL_GTLDS_SECTION_HEADER = "// newGTLDs"
	// PSL_GTLDS_SECTION_FOOTER marks the end of the newGTLDs section of the
	// overall public suffix dat file.
	PSL_GTLDS_SECTION_FOOTER = "// ===END ICANN DOMAINS==="
)

var (
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
	// GTLDEntry objects in the format used by the public suffix list.
	//
	// It expects the following template data:
	//   Entries - a list of GTLDEntry objects.
	pslGTLDTemplate = template.Must(
		template.New("public-suffix-list-gtlds").Parse(`
{{- range .Entries }}
{{- .Comment }}
{{ printf "%s\n" .ULabel }}
{{ end }}`))
)

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
		URL:        icann.GTLD_JSON_REGISTRY_URL,
		Date:       clk.Now().UTC(),
		DateFormat: time.RFC3339,
	}

	return renderTemplate(writer, pslGTLDHeaderTemplate, templateData)
}

// renderGTLDData renders the given list of icann.GTLDEntry objects using the
// pslGTLDTemplate. The rendered template data is written to the provided writer
// or an error is returned.
func renderGTLDData(writer io.Writer, entries []*icann.GTLDEntry) error {
	templateData := struct {
		Entries []*icann.GTLDEntry
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
	entries, err := icann.GetGTLDs(dataURL)
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
