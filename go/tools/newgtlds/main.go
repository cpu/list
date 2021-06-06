// newgtlds is a utility command that downloads the list of gTLDs from ICANN
// and formats it into the PSL format, writing to stdout.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"text/template"
	"time"

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

// getData performs a HTTP GET request to the given URL and returns the
// response body bytes or returns an error. An HTTP response code other than
// http.StatusOK (200) is considered to be an error.
func getData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code fetching data "+
			"from %q : expected status %d got %d",
			url, http.StatusOK, resp.StatusCode)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
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

	// Process the dat file to update GTLDs based on ICANN_GTLD_JSON_URL data.
	content, err := processGTLDs(datFile, ICANN_GTLD_JSON_URL, nil)
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
