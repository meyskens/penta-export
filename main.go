package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/src-d/envconfig"
	"golang.org/x/net/html"
)

// Config contains the info needed to run the system
type Config struct {
	PentaURL  string `default:"https://penta.fosdem.org"`
	Username  string `required:"true"`
	Password  string `required:"true"`
	DevroomID string `required:"true"`
}

// TalkInfo is the info of a talk proposal
type TalkInfo struct {
	ID          string
	Title       string
	Notes       string
	Subtitle    string
	Abstract    string
	Description string
}

var config Config

func main() {
	err := envconfig.Process("penta", &config)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}
	if config.PentaURL == "" { // default isn't working for a weird reason :(
		config.PentaURL = "https://penta.fosdem.org"
	}

	csv, err := getCSV()
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}

	fmt.Println("ID,Title,Subtitle,Abstract,Description,Notes")
	for _, line := range strings.Split(string(csv), "\n") {
		data := strings.Split(line, ",")
		if len(data) < 2 { // invalid line
			continue
		}
		if data[0] == "ID" {
			continue
		}
		talk, _ := getTalk(data[0])
		fmt.Printf(`%s,"%s","%s","%s","%s","%s"\n`, csvFriendlify(talk.ID), csvFriendlify(talk.Title), csvFriendlify(talk.Subtitle), csvFriendlify(talk.Abstract), csvFriendlify(talk.Description), csvFriendlify(talk.Notes))
	}

}

func getCSV() ([]byte, error) {
	resp, err := doRequest("/search/search_event_advanced", map[string]string{
		"search_event[0][key]":   "conference_track_id",
		"search_event[0][type]":  "list",
		"search_event[0][value]": config.DevroomID,
	})

	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}

	r := html.NewTokenizer(resp.Body)
	csvURL := ""
L:
	for {
		tt := r.Next()
		if tt == html.ErrorToken {
			return nil, r.Err()
		}
		token := r.Token()
		if token.Data == "a" {
			for _, attr := range token.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, "csv") {
					csvURL = attr.Val
					break L
				}
			}
		}
	}
	resp, err = doRequest(csvURL, nil)
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}

	body, _ := ioutil.ReadAll(resp.Body)

	return body, nil
}

func getTalk(id string) (*TalkInfo, error) {
	resp, err := doRequest(fmt.Sprintf("/event/edit/%s", id), nil)
	if err != nil {
		return nil, err
	}
	info := TalkInfo{
		ID: id,
	}

	r := html.NewTokenizer(resp.Body)
	for {
		tt := r.Next()
		if tt == html.ErrorToken {
			break
		}
		token := r.Token()
		if token.Data == "input" || token.Data == "textarea" {
			var to *string
			value := ""
			for _, attr := range token.Attr {
				if attr.Key == "value" {
					value = attr.Val
				} else if attr.Key == "id" && attr.Val == "event[title]" {
					to = &info.Title
				} else if attr.Key == "id" && attr.Val == "event[subtitle]" {
					to = &info.Subtitle
				} else if attr.Key == "id" && attr.Val == "event[abstract]" {
					to = &info.Abstract
				} else if attr.Key == "id" && attr.Val == "event[description]" {
					to = &info.Description
				} else if attr.Key == "id" && attr.Val == "event[submission_notes]" {
					to = &info.Notes
				}
			}
			if value != "" && to != nil {
				// found useful info in <input>
				*to = value
			}
			if value == "" && to != nil {
				// found useful info in <textarea>
				r.Next()
				*to = r.Token().String()
				if *to == "</textarea>" {
					*to = "" // no content
				}
				if *to == "</td>" {
					*to = "" // no content
				}
			}
		}
	}

	return &info, nil
}

func csvFriendlify(in string) string {
	return strings.Replace(strings.Replace(in, "\"", "\"\",", -1), "\n", "\\n", -1)
}

func doRequest(url string, query map[string]string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s", config.PentaURL, url), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	if query != nil {
		for key, value := range query {
			q.Add(key, value)
		}
	}
	req.URL.RawQuery = q.Encode()
	req.SetBasicAuth(config.Username, config.Password)
	return client.Do(req)
}
