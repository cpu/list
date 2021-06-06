package main

import (
	"io/ioutil"
	"strings"
)

// gTLDDatSpan represents the span between the PSL_GTLD_SECTION_HEADER and
// the PSL_GTLDS_SECTION_FOOTER in the PSL dat file.
type gTLDDatSpan struct {
	startIndex int
	endIndex   int
}

// validate checks that a given gTLDDatSpan is sensible. It returns an err if
// the span is nil, if the start or end index haven't been set to > 0, or if the
// end index is <= the the start index.
func (s gTLDDatSpan) validate() error {
	if s.startIndex <= 0 {
		return errNoHeader
	}
	if s.endIndex <= 0 {
		return errNoFooter
	}
	if s.endIndex <= s.startIndex {
		return errInvertedSpan{span: s}
	}
	return nil
}

// datFile holds the individual lines read from the public suffix list dat file and
// the span that holds the gTLD specific data section. It supports reading the
// gTLD specific data, and replacing it.
type datFile struct {
	// lines holds the datfile contents split by "\n"
	lines []string
	// gTLDSpan holds the indexes where the gTLD data can be found in lines.
	gTLDSpan gTLDDatSpan
}

// validate validates the state of the datFile. It returns an error if
// the gTLD span validate() returns an error, or if gTLD span endIndex is >= the
// number of lines in the file.
func (d datFile) validate() error {
	if err := d.gTLDSpan.validate(); err != nil {
		return err
	}
	if d.gTLDSpan.endIndex >= len(d.lines) {
		return errSpanOutOfBounds{span: d.gTLDSpan, numLines: len(d.lines)}
	}
	return nil
}

// getGTLDLines returns the lines from the dat file within the gTLD data span,
// or an error if the span isn't valid for the dat file.
func (d datFile) getGTLDLines() ([]string, error) {
	if err := d.validate(); err != nil {
		return nil, err
	}
	return d.lines[d.gTLDSpan.startIndex:d.gTLDSpan.endIndex], nil
}

// ReplaceGTLDContent updates the dat file's lines to replace the gTLD data span
// with new content.
func (d *datFile) ReplaceGTLDContent(content string) error {
	if err := d.validate(); err != nil {
		return err
	}

	contentLines := strings.Split(content, "\n")
	beforeLines := d.lines[0:d.gTLDSpan.startIndex]
	afterLines := d.lines[d.gTLDSpan.endIndex:]
	newLines := append(beforeLines, append(contentLines, afterLines...)...)

	// Update the span based on the new content length
	d.gTLDSpan.endIndex = len(beforeLines) + len(contentLines)
	// and update the data file lines
	d.lines = newLines
	return nil
}

// String returns the dat file's lines joined together.
func (d datFile) String() string {
	return strings.Join(d.lines, "\n")
}

// readDatFile reads the contents of the PSL dat file from the provided path
// and returns a representation holding all of the lines and the span where the gTLD
// data is found within the dat file. An error is returned if the file can't be read
// or if the gTLD data span can't be found or is invalid.
func readDatFile(datFilePath string) (*datFile, error) {
	pslDatBytes, err := ioutil.ReadFile(datFilePath)
	if err != nil {
		return nil, err
	}
	return readDatFileContent(string(pslDatBytes))
}

func readDatFileContent(pslData string) (*datFile, error) {
	pslDatLines := strings.Split(pslData, "\n")

	headerIndex, footerIndex := 0, 0
	for i := 0; i < len(pslDatLines); i++ {
		line := pslDatLines[i]

		if line == PSL_GTLDS_SECTION_HEADER && headerIndex == 0 {
			// If the line matches the header and we haven't seen the header yet, capture
			// the index
			headerIndex = i
		} else if line == PSL_GTLDS_SECTION_HEADER && headerIndex != 0 {
			// If the line matches the header and we've already seen the header return
			// an error. This is unexpected.
			return nil, errMultipleHeaders
		} else if line == PSL_GTLDS_SECTION_FOOTER && footerIndex == 0 {
			// If the line matches the footer, capture the index. We don't need
			// to consider the case where we've already seen a footer because we break
			// below when we have both a header and footer index.
			footerIndex = i
		}

		// Break when we have found one header and one footer.
		if headerIndex != 0 && footerIndex != 0 {
			break
		}
	}

	if headerIndex == 0 {
		return nil, errNoHeader
	} else if footerIndex == 0 {
		return nil, errNoFooter
	}

	datFile := &datFile{
		lines: pslDatLines,
		gTLDSpan: gTLDDatSpan{
			startIndex: headerIndex + 1,
			endIndex:   footerIndex,
		},
	}
	if err := datFile.validate(); err != nil {
		return nil, err
	}

	return datFile, nil
}
