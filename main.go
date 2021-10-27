package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var parent = ""
var urls sync.Map
var uniqueUrls = 0

func ProcessHref(href string, found chan []string) {
	if len(href) == 0 {
		return
	}

	if strings.HasPrefix(href, "/") {
		href = fmt.Sprintf("https://%s%s", parent, href)
	}

	// Request the HTML page.
	res, err := http.Get(href)

	if err != nil {
		log.Err(err).Msg("http request error")
		return
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Error().
			Str("href", href).
			Int("status-code", res.StatusCode).
			Str("status", res.Status).
			Msg("status code not 200")
		return
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)

	if err != nil {
		log.Err(err).
			Str("href", href).
			Msg("new document read error")
		return
	}

	anchors := doc.Find("a")

	log.Info().
		Str("href", href).
		Int("anchorCount", len(anchors.Nodes)).
		Msg("new page anchor count")

	hrefs := []string{}

	// Find the anchors
	anchors.Each(func(i int, s *goquery.Selection) {
		// For each anchor found, get the href
		href, hasHref := s.Attr("href")

		// For now, we don't care about query params
		href = strings.Split(href, "?")[0]

		if _, exists := urls.Load(href); exists || !hasHref || !strings.Contains(href, parent) {
			return
		}

		if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "http") {
			urls.Store(href, struct{}{})
			uniqueUrls++

			log.Info().
				Str("href", href).
				Int("uniqueUrls", uniqueUrls).
				Msg("found new href")

			hrefs = append(hrefs, href)
		}
	})

	log.Info().
		Str("href", href).
		Int("hrefs", len(hrefs)).
		Msg("new hrefs count")

	if len(hrefs) > 0 {
		found <- hrefs
	}
}

func StartScrape(queue chan string, found chan []string, i int) {
	for {
		href := <-queue

		ProcessHref(href, found)

		log.Info().
			Str("href", href).
			Int("routine", i).
			Msg("finished processing")
	}
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	var next string
	found := make(chan []string)
	queue := make(chan string)

	parent = "amazon.com"

	for i := range [4]int{} {
		go StartScrape(queue, found, i)
	}

	items := []string{fmt.Sprintf("https://%s", parent)}

	for {
		if len(items) > 0 {
			next = items[0]
			items = items[1:]
		} else {
			next = ""
			items = []string{}
		}

		// Since our work list is empty we don't have the next href to put on the queue.
		// We should wait for the work list to be populated again instead of spinning.
		if len(next) == 0 {
			hrefs := <-found
			items = append(items, hrefs...)
		} else {
			select {
			case hrefs := <-found:
				items = append(items, hrefs...)
			case queue <- next:
			}
		}
	}
}
