package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

const filmwebURL = "www.filmweb.pl"

func getChannelEPG(channelKey string) []epgEntry {
	epgEntries := []epgEntry{}
	results := []epgEntry{}
	c := colly.NewCollector(colly.AllowedDomains(filmwebURL))

	c.OnHTML("div", func(e *colly.HTMLElement) {
		if e.Attr("data-sid") != "" {
			filmID, err := strconv.Atoi(e.Attr("data-film"))
			if err != nil {
				filmID = 0
			}
			sid, err := strconv.Atoi(e.Attr("data-sid"))
			if err != nil {
				sid = 0
			}

			link := e.ChildAttr("a", "href")
			var year int
			if strings.HasPrefix(link, "/film/") {
				linkSlice := strings.Split(link, "/")
				titleYearID := strings.Split(linkSlice[2], "-")
				year, err = strconv.Atoi(titleYearID[1])
				if err != nil {
					year = 0
				}
			}
			newEntry := epgEntry{
				title:   e.ChildText("a"),
				year:    year,
				filmID:  filmID,
				sid:     sid,
				start:   e.Attr("data-start"),
				stop:    "",
				isMovie: strings.Contains(e.Attr("class"), "film"),
			}

			fmt.Printf("%+v\n", newEntry)

			epgEntries = append(epgEntries, newEntry)
		}
	})

	c.Visit(fmt.Sprintf("https://%s/program-tv/%s", filmwebURL, channelKey))

	for i := 0; i < len(epgEntries); i++ {
		if i < len(epgEntries)-1 {
			epgEntries[i].stop = epgEntries[i+1].start
		}
		if epgEntries[i].isMovie && epgEntries[i].title != "" {
			results = append(results, epgEntries[i])
		}
	}
	return results
}
