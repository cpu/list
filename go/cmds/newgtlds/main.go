// newgtlds is a utility command that downloads the list of gTLDs from ICANN
// and formats it into the PSL format, writing to stdout.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"github.com/publicsuffix/list/go/datasource/icann"
	pslGo "github.com/weppos/publicsuffix-go/publicsuffix"
)

// clock is a small interface that lets us mock time in unit tests.
type clock interface {
	Now() time.Time
}

// realClock is an implementation of clock that uses time.Now() natively.
type realClock struct{}

// Now returns the current time.Time using the system clock.
func (c realClock) Now() time.Time {
	return time.Now()
}

// renderTemplate renders the given template to the provided writer, using the
// templateData, or returns an error.
func renderTemplate(writer io.Writer, template *template.Template, templateData interface{}) error {
	var buf bytes.Buffer
	if err := template.Execute(&buf, templateData); err != nil {
		return err
	}

	_, err := writer.Write(buf.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func main() {
	ifErrQuit := func(err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "error updating gTLD data: %v\n", err)
			os.Exit(1)
		}
	}

	pslDatFile := flag.String(
		"psl-dat-file",
		"public_suffix_list.dat",
		"file path to the public_suffix.dat data file to be updated with new gTLDs")

	overwrite := flag.Bool(
		"overwrite",
		false,
		"overwrite -psl-dat-file with the new data instead of printing to stdout")

	// Parse CLI flags.
	flag.Parse()

	// Read the existing file content and find the span that contains the gTLD data.
	datFile, err := readDatFile(*pslDatFile)
	ifErrQuit(err)

	// Process the dat file to update GTLDs based on the ICANN
	// GTLD_JSON_REGISTRY_URL data.
	content, err := processGTLDs(datFile, icann.GTLD_JSON_REGISTRY_URL, nil)
	ifErrQuit(err)

	pslList, err := pslGo.NewListFromString(content, nil)
	ifErrQuit(err)
	fmt.Printf("PslList Size: %v\n", pslList.Size())

	// If we're not overwriting the file, print the content to stdout.
	if !*overwrite {
		fmt.Println(content)
		os.Exit(0)
	}

	// Otherwise print nothing to stdout and write the content over the exiting
	// pslDatFile path we read earlier.
	err = ioutil.WriteFile(*pslDatFile, []byte(content), 0644)
	ifErrQuit(err)
}
