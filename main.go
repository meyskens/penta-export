package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/net/html"
)

// Config contains the info needed to run the system
type Config struct {
	PentaURL  string `default:"https://penta.fosdem.org"`
	Username  string `required:"true"`
	Password  string `required:"true"`
	DevroomID string `required:"true" envconfig:"devroom_id"`
}

// TalkInfo is the info of a talk proposal
type TalkInfo struct {
	ID          string
	Title       string
	Notes       string
	Subtitle    string
	Abstract    string
	Description string
	Duration    string
	State       string
	Progress    string
	PersonID    string
	StartTime   string
}

// PersonInfo is the info the talk submitter
type PersonInfo struct {
	FirstName string
	LastName  string
	Email     string
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

	csvdata, err := getCSV()
	if err != nil {
		log.Fatal(err.Error())
		os.Exit(1)
	}

	fmt.Println("ID,Title,Subtitle,Abstract,Description,Notes,Duration,State,Progress,FirstName,Email,LastName,StartTime")
	r := csv.NewReader(strings.NewReader(string(csvdata[:])))
	for {
		data, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		if len(data) < 2 { // invalid line
			continue
		}
		if data[0] == "ID" {
			continue
		}
		talk, _ := getTalk(data[0])
		talk.Duration = data[7]
		talk.StartTime = strings.TrimSuffix(data[5], ":00")
		person, _ := getPerson(talk.PersonID)
		fmt.Printf("%s,\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"\n", csvFriendlify(talk.ID), csvFriendlify(talk.Title), csvFriendlify(talk.Subtitle), csvFriendlify(talk.Abstract), csvFriendlify(talk.Description), csvFriendlify(talk.Notes), csvFriendlify(talk.Duration), csvFriendlify(talk.State), csvFriendlify(talk.Progress), csvFriendlify(person.FirstName), csvFriendlify(person.Email), csvFriendlify(person.LastName), csvFriendlify(talk.StartTime))
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
				*to = html.UnescapeString(value)
			}
			if value == "" && to != nil {
				// found useful info in <textarea>
				r.Next()
				*to = html.UnescapeString(r.Token().String())
				if *to == "</textarea>" {
					*to = "" // no content
				}
				if *to == "</td>" {
					*to = "" // no content
				}
			}
		}

		if strings.Contains(token.Data, "add_event_person") {
			ops := strings.Split(token.Data, ";")
			for _, op := range ops {
				if ! strings.Contains(op, "add_event_person") {
					continue
				}
				parts := strings.Split(op, ",")
				id := strings.Replace(parts[2], "'", "", -1)
				if parts[3] == "'speaker'" {
					info.PersonID = id
				}
			}
		}

		if token.Data == "select" {
			var to *string
			found := false
			value := ""
			for _, attr := range token.Attr {
				if attr.Key == "id" && attr.Val == "event[event_state]" {
					to = &info.State
					found = true
				} else if attr.Key == "id" && attr.Val == "event[event_state_progress]" {
					to = &info.Progress
					found = true
				}
			}
			if !found {
				continue
			}

			for {
				tt = r.Next()
				if tt == html.ErrorToken {
					break
				}
				token = r.Token()

				selected := false
				val := ""
				for _, attr := range token.Attr {
					if attr.Key == "selected" && attr.Val == "selected" {
						selected = true
					} else if attr.Key == "value" {
						val = attr.Val
					}
				}
				if selected && val != "" {
					value = val
				}

				if token.Data == "select" && token.Type == html.EndTagToken {
					break
				}
			}

			if value != "" && to != nil {
				*to = html.UnescapeString(value)
			}
		}
	}

	return &info, nil
}

func getPerson(id string) (*PersonInfo, error) {
	resp, err := doRequest(fmt.Sprintf("/person/edit/%s", id), nil)
	if err != nil {
		return nil, err
	}
	info := PersonInfo{}

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
				} else if attr.Key == "id" && attr.Val == "person[first_name]" {
					to = &info.FirstName
				} else if attr.Key == "id" && attr.Val == "person[last_name]" {
					to = &info.LastName
				} else if attr.Key == "id" && attr.Val == "person[email]" {
					to = &info.Email
				}
			}
			if value != "" && to != nil {
				// found useful info in <input>
				*to = html.UnescapeString(value)
			}
			if value == "" && to != nil {
				// found useful info in <textarea>
				r.Next()
				*to = html.UnescapeString(r.Token().String())
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
