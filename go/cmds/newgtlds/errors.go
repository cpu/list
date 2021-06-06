package main

import "fmt"

var (
	errNoHeader = fmt.Errorf("did not find expected header line %q",
		PSL_GTLDS_SECTION_HEADER)
	errMultipleHeaders = fmt.Errorf("found expected header line %q more than once",
		PSL_GTLDS_SECTION_HEADER)
	errNoFooter = fmt.Errorf("did not find expected footer line %q",
		PSL_GTLDS_SECTION_FOOTER)
)

type errInvertedSpan struct {
	span gTLDDatSpan
}

func (e errInvertedSpan) Error() string {
	return fmt.Sprintf(
		"found footer line %q before header line %q (index %d vs %d)",
		PSL_GTLDS_SECTION_FOOTER, PSL_GTLDS_SECTION_HEADER,
		e.span.endIndex, e.span.startIndex)
}

type errSpanOutOfBounds struct {
	span     gTLDDatSpan
	numLines int
}

func (e errSpanOutOfBounds) Error() string {
	return fmt.Sprintf(
		"span out of bounds: start index %d, end index %d, number of lines %d",
		e.span.startIndex, e.span.endIndex, e.numLines)
}
