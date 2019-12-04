package main

import (
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type glRunner struct {
	ID          uint64 `json:"id"`
	Description string `json:"description"`
}

var client http.Client
var token string

func main() {
	baseUrl := flag.String("baseurl", "", "https://gitlab.example.com/")
	pattern := flag.String("pattern", "", "test")
	force := flag.Bool("force", false, "")
	flag.Usage = usage

	flag.Parse()

	if *baseUrl == "" {
		fmt.Fprintln(os.Stderr, "base URL missing")
		usage()
		os.Exit(2)
	}

	if *pattern == "" {
		fmt.Fprintln(os.Stderr, "pattern missing")
		usage()
		os.Exit(2)
	}

	token = os.Getenv("TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "token missing")
		usage()
		os.Exit(2)
	}

	rootURL, errURL := url.Parse(*baseUrl)
	if errURL != nil {
		fmt.Fprintf(os.Stderr, "bad base URL: %s\n", errURL.Error())
		usage()
		os.Exit(2)
	}

	rgx, errRC := regexp.Compile(*pattern)
	if errRC != nil {
		fmt.Fprintf(os.Stderr, "bad pattern: %s\n", errRC.Error())
		usage()
		os.Exit(2)
	}

	if !strings.HasSuffix(rootURL.Path, "/") {
		rootURL.Path += "/"
		rootURL.RawPath += "/"
	}

	allRunners := map[string]uint64{}
	runnersURL := rootURL.ResolveReference(parseURL("api/v4/runners/"))

	{
		var page uint64 = 1

		for {
			runnersURL.RawQuery = fmt.Sprintf("page=%d", page)

			var runners []glRunner
			assert(getJson(runnersURL, &runners))

			if len(runners) < 1 {
				break
			}

			for _, runner := range runners {
				if rgx.MatchString(runner.Description) {
					allRunners[runner.Description] = runner.ID
				}
			}

			page++
		}
	}

	if *force {
		runnersURL.RawQuery = ""

		for _, id := range allRunners {
			_, errReq := req("DELETE", runnersURL.ResolveReference(parseURL(strconv.FormatUint(id, 10))))
			assert(errReq)
		}
	} else {
		for desc, id := range allRunners {
			log.WithFields(log.Fields{
				"id":          id,
				"description": desc,
			}).Info("Not removed runner")
		}
	}
}

func parseURL(uri string) *url.URL {
	urAL, errURL := url.Parse(uri)
	if errURL != nil {
		panic(errURL)
	}

	return urAL
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: TOKEN=123456 %s -baseurl https://gitlab.example.com/ -pattern test [-force]\n", os.Args[0])
}

func assert(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func getJson(urAL *url.URL, jsn interface{}) error {
	body, errReq := req("GET", urAL)
	if errReq != nil {
		return errReq
	}

	return json.Unmarshal(body, jsn)
}

func req(method string, urAL *url.URL) ([]byte, error) {
	log.WithFields(log.Fields{
		"method":  method,
		"url":     urAL,
		"headers": map[string]interface{}{"Private-Token": "***"},
	}).Info("performing HTTP request")

	res, errReq := client.Do(&http.Request{Method: method, URL: urAL, Header: map[string][]string{"Private-Token": {token}}})
	if errReq != nil {
		return nil, errReq
	}

	defer res.Body.Close()

	if res.StatusCode > 299 {
		return nil, fmt.Errorf("got HTTP status %d", res.StatusCode)
	}

	body, errRA := ioutil.ReadAll(res.Body)
	if errRA != nil {
		return nil, errRA
	}

	log.WithFields(log.Fields{
		"method":          method,
		"url":             urAL,
		"request_headers": map[string]interface{}{"Private-Token": "***"},
		"body":            string(body),
	}).Debug("got HTTP response")

	return body, nil
}
