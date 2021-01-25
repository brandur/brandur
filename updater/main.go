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
		readmeData.Articles, err = getAtomFeedEntries(baseURL + "/articles.atom")
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
		readmeData.Fragments, err = getAtomFeedEntries(baseURL + "/fragments.atom")
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
		readmeData.Nanoglyphs, err = getAtomFeedEntries(baseURL + "/nanoglyphs.atom")
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

// A backoff schedule for when and how often to retry failed HTTP requests. The
// first element is the time to wait after the first failure, the second the
// time to wait after the second failure, etc. After reaching the last element,
// retries stop and the request is considered failed.
var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	10 * time.Second,
}

var localLocation *time.Location = mustLocation("America/Los_Angeles")

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
	Summary   string    `xml:"summary"`
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

func formatTimeLocal(t time.Time) string {
	return t.In(localLocation).Format("January 2, 2006")
}

func getAtomFeedEntries(url string) ([]*Entry, error) {
	resp, body, err := getURLDataWithRetries(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 status code fetching URL '%s': %v",
			url, string(body))
	}

	var feed Feed

	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling Atom feed XML: %w", err)
	}

	return feed.Entries, nil
}

// Gets data at a URL. Connects and reads the entire response string, but
// notably does not check for problems with bad status codes.
func getURLData(url string) (*http.Response, []byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching URL '%s': %w", url, err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading response body from URL '%s': %w", url, err)
	}

	return resp, body, nil
}

func getURLDataWithRetries(url string) (*http.Response, []byte, error) {
	var body []byte
	var err error
	var resp *http.Response

	for _, backoff := range backoffSchedule {
		resp, body, err = getURLData(url)

		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, "Request error: %+v\n", err)
		fmt.Fprintf(os.Stderr, "Retrying in %v\n", backoff)
		time.Sleep(backoff)
	}

	// All retries failed
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func mustLocation(locationName string) *time.Location {
	locatio, err := time.LoadLocation(locationName)
	if err != nil {
		panic(err)
	}
	return locatio
}

func renderTemplateToStdout(readmeData *READMEData) error {
	readmeTemplate := template.Must(
		template.New("").Funcs(template.FuncMap{
			"FormatTimeLocal": formatTimeLocal,
		}).ParseFiles("README.md.tmpl"),
	)

	err := readmeTemplate.ExecuteTemplate(os.Stdout, "README.md.tmpl", readmeData)
	if err != nil {
		return fmt.Errorf("error rendering README.md template: %w", err)
	}

	return nil
}
