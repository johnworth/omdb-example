package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// SearchRequest represents the variables that are passed to the OMDb API.
type SearchRequest struct {
	Title       string `json:"title"` // This is the only required field for the API.
	Type        string `json:"type,omitempty"`
	ReleaseYear string `json:"release_year,omitempty"`
	APIVersion  string `json:"api_verison"`
}

// SearchResult represents the variables that are returned by the OMDb API.
type SearchResult struct {
	Title  string
	Year   string
	IMDBID string
	Type   string
}

// SearchWrapper is the outer-wrapper around the search results returned by
// the API.
type SearchWrapper struct {
	Search []*SearchResult
}

// NewSearchRequest returns a *SearchRequest populated with default values for
// the OMDb API request.
func NewSearchRequest(title string) *SearchRequest {
	return &SearchRequest{
		Title:      title,
		APIVersion: "1",
	}
}

// API is the interface for making requests against a remote api.
type API interface {
	Init(key string) API
	Search(*SearchRequest) ([]*SearchResult, error)
}

// OMDBAPI is a concrete implementation of the API interface that interacts with
// the Open Movie Database, located at https://www.omdbapi.com.
type OMDBAPI struct {
	url *url.URL
}

// Init will return a newly instantiated OMDBAPI instance.
func Init(key string) (*OMDBAPI, error) {
	u, err := url.Parse("http://www.omdbapi.com/?")
	if err != nil {
		return nil, err
	}

	v := u.Query()
	v.Set("apikey", key)
	u.RawQuery = v.Encode()

	return &OMDBAPI{
		url: u,
	}, nil
}

// searchURL returns a *url.URL based with the correct values in the query
// string.
func (o *OMDBAPI) searchURL(r *SearchRequest) *url.URL {
	n := *o.url
	v := n.Query()

	v.Set("s", r.Title)

	if r.Type != "" {
		v.Set("type", r.Type)
	}

	if r.ReleaseYear != "" {
		v.Set("y", r.ReleaseYear)
	}

	n.RawQuery = v.Encode()
	return &n
}

// Search calls the OMDBAPI and returns a *SearchResult.
func (o *OMDBAPI) Search(r *SearchRequest) ([]*SearchResult, error) {
	searchURL := o.searchURL(r)
	resp, err := http.Get(searchURL.String())
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result *SearchWrapper
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Search, nil
}

// App interface defines the base functionality that a type must support to be
// considered an App.
type App interface {
	Home(http.ResponseWriter, *http.Request)
	Search(http.ResponseWriter, *http.Request)
}

// SearchApp implements the App interface for sending handling requests from
// the frontend.
type SearchApp struct {
	searchAPI *OMDBAPI
	mux       *http.ServeMux
}

// NewSearchApp returns a new *SearchApp.
func NewSearchApp(key string) (*SearchApp, error) {
	api, err := Init(key)
	if err != nil {
		return nil, err
	}

	m := http.NewServeMux()
	s := &SearchApp{
		searchAPI: api,
		mux:       m,
	}
	s.mux.Handle("/", http.FileServer(http.Dir("site")))
	s.mux.HandleFunc("/search", s.Search)
	return s, nil
}

// Home handles requests to /.
func (s *SearchApp) Home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "search.html")
}

// Search handles requests to /search
func (s *SearchApp) Search(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var searchRequest *SearchRequest
	if err = json.Unmarshal(b, &searchRequest); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results, err := s.searchAPI.Search(searchRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonstr, err := json.Marshal(results)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(jsonstr)
}

func fixAddr(addr string) string {
	if !strings.HasPrefix(addr, ":") {
		return fmt.Sprintf(":%s", addr)
	}
	return addr
}

func main() {
	var (
		key  = flag.String("key", "", "The OMDb API key.")
		port = flag.String("port", "60000", "The port number to listen on.")
	)

	flag.Parse()

	if *key == "" {
		fmt.Println("--key is required.")
		os.Exit(-1)
	}

	app, err := NewSearchApp(*key)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(http.ListenAndServe(fixAddr(*port), app.mux))
}
