package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

func main() {
	var readmeData READMEData
	var wg sync.WaitGroup

	//
	// Articles
	//
	wg.Add(1)
	go func() {
		var err error
		readmeData.Articles, err = getAtomFeedItems(baseURL + "/articles.atom")
		if err != nil {
			fail(err)
		}

		wg.Done()
	}()

	//
	// Fragments
	//
	wg.Add(1)
	go func() {
		var err error
		readmeData.Fragments, err = getAtomFeedItems(baseURL + "/fragments.atom")
		if err != nil {
			fail(err)
		}

		wg.Done()
	}()

	//
	// Nanoglyphs
	//
	wg.Add(1)
	go func() {
		var err error
		readmeData.Nanoglyphs, err = getAtomFeedItems(baseURL + "/nanoglyphs.atom")
		if err != nil {
			fail(err)
		}

		// Massage titles slightly to remove the leading "Nanoglyph"
		for _, entry := range readmeData.Nanoglyphs {
			entry.Title = strings.Replace(entry.Title, "Nanoglyph ", "", 1)
		}

		wg.Done()
	}()

	wg.Wait()

	err := renderTemplateToStdout(&readmeData)
	if err != nil {
		fail(err)
	}
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

const baseURL = "https://brandur.org"

// Feed represents an Atom feed. Used for deserializing XML.
type Feed struct {
	XMLName xml.Name `xml:"feed"`

	Entries []*Entry `xml:"entry"`
	Title   string   `xml:"title"`
}

// Entry represents an entry in an Atom feed. Used for deserializing XML.
type Entry struct {
	Link struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Title     string    `xml:"title"`
	Published time.Time `xml:"published"`
}

// READMEData is a struct containing all the information necessary to render a
// new version of `README.md`.
type READMEData struct {
	Articles   []*Entry
	Fragments  []*Entry
	Nanoglyphs []*Entry
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error during execution:\n%v\n", err)
	os.Exit(1)
}

// Gets data at a URL. Connects and reads the entire response string, but
// notably does not check for problems with bad status codes.
func getURLData(url string) (*http.Response, []byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Error fetching URL '%s'", url)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Error reading response body from URL '%s'", url)
	}

	return resp, body, nil
}

func getAtomFeedItems(url string) ([]*Entry, error) {
	var body []byte
	var err error
	var resp *http.Response

	// A very simple backoff loop that tries to make the build more stable by
	// retrying transient errors so that I won't get emailed about them. (This
	// is mostly a prospective improvement. I haven't observed this happening,
	// but expect that it might occasionally.)
	for _, backoffSecs := range []int{1, 3, 10} {
		resp, body, err = getURLData(url)

		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, "%+v\n", err)
		fmt.Fprintf(os.Stderr, "Retrying in %v second(s)\n", backoffSecs)
		time.Sleep(time.Duration(backoffSecs) * time.Second)
	}

	// All retries failed
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "Non-200 status code fetching URL '%s': %v",
			url, string(body))
	}

	var feed Feed

	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, errors.Wrapf(err, "Error unmarshaling Atom feed XML")
	}

	return feed.Entries, nil
}

func renderTemplateToStdout(readmeData *READMEData) error {
	readmeTemplate := template.Must(
		template.New("").ParseFiles("README.md.tmpl"),
	)

	err := readmeTemplate.ExecuteTemplate(os.Stdout, "README.md.tmpl", readmeData)
	if err != nil {
		return errors.Wrap(err, "Error rendering README.md template")
	}

	return nil
}
