package datasource

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetHTTPData performs a HTTP GET request to the given URL and returns the
// response body bytes or returns an error. An HTTP response code other than
// http.StatusOK (200) is considered to be an error.
func GetHTTPData(url string) ([]byte, error) {
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
