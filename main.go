package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jordic/goics"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

const feedPrefix = "/feed/"
const expirationTime = 5 * time.Minute

// Feed is an iCal feed
type Feed struct {
	Content   string
	ExpiresAt time.Time
}

// Entry is a time entry
type Entry struct {
	DateStart   time.Time `json:"dateStart"`
	DateEnd     time.Time `json:"dateEnd"`
	Description string    `json:"description"`
}

// Entries is a collection of entries
type Entries []*Entry

func main() {
	cache := make(map[string]*Feed)

	mux := http.NewServeMux()
	mux.HandleFunc("/feedURL", feedURL(cache))
	mux.HandleFunc(feedPrefix, feed(cache))

	log.Print("Server started on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func feedURL(cache map[string]*Feed) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := randomToken(20)
		_, err := createFeedForToken(token, cache)
		if err != nil {
			writeError(http.StatusInternalServerError, "Could not create feed", w, err)
			return
		}
		writeSuccess(fmt.Sprintf("FeedToken: %s", token), w)
	})
}

func feed(cache map[string]*Feed) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "text/calendar")
		w.Header().Set("charset", "utf-8")
		w.Header().Set("Content-Disposition", "inline")
		w.Header().Set("filename", "calendar.ics")

		var result string
		token := parseToken(r.URL.Path)
		log.Print("Fetching iCal feed for Token: " + token)
		feed, ok := cache[token]
		if !ok || feed == nil {
			writeError(http.StatusNotFound, "No Feed for this Token", w, errors.New("No Feed for this Token"))
			return
		}

		result = feed.Content
		if feed.ExpiresAt.Before(time.Now()) {
			newFeed, err := createFeedForToken(token, cache)
			if err != nil {
				writeError(http.StatusInternalServerError, "Could not create feed", w, err)
				return
			}
			result = newFeed.Content
		}

		writeSuccess(result, w)
	})
}

func createFeedForToken(token string, cache map[string]*Feed) (*Feed, error) {
	res, err := fetchData()
	if err != nil {
		return nil, errors.New("Could not fetch data")
	}
	b := bytes.Buffer{}
	goics.NewICalEncode(&b).Encode(res)
	feed := &Feed{Content: b.String(), ExpiresAt: time.Now().Add(expirationTime)}
	cache[token] = feed
	return feed, nil
}

func fetchData() (Entries, error) {
	url := "http://www.mocky.io/v2/5a88375b3000007e007f9401"
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.New("could not fetch data")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %s", "could not fetch data", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("could not read data")
	}
	result := Entries{}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, errors.New("could not unmarshal data")
	}
	return result, nil

}

// EmitICal implements the interface for goics
func (e Entries) EmitICal() goics.Componenter {
	c := goics.NewComponent()
	c.SetType("VCALENDAR")
	c.AddProperty("CALSCAL", "GREGORIAN")
	for _, entry := range e {
		s := goics.NewComponent()
		s.SetType("VEVENT")
		k, v := goics.FormatDateTimeField("DTEND", entry.DateEnd)
		s.AddProperty(k, v)
		k, v = goics.FormatDateTimeField("DTSTART", entry.DateStart)
		s.AddProperty(k, v)
		s.AddProperty("SUMMARY", entry.Description)

		c.AddComponent(s)
	}
	return c
}

func parseToken(path string) string {
	return strings.TrimPrefix(path, feedPrefix)
}

func randomToken(len int) string {
	b := make([]byte, len)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func writeError(status int, message string, w http.ResponseWriter, err error) {
	log.Print("ERROR: ", err.Error())
	w.WriteHeader(status)
	w.Write([]byte(message))
}

func writeSuccess(message string, w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}
