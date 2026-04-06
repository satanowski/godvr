package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/gocolly/colly"
)

const filmwebURL = "www.filmweb.pl"

func getChannelEPG(channelKey string) ([]epgEntry, error) {
	var results []epgEntry
	c := colly.NewCollector(colly.AllowedDomains(filmwebURL))

	c.OnHTML("div.tvPageGroupedSeances", func(e *colly.HTMLElement) {
		if e.Attr("data-entity-name") != "film" {
			return
		}

		startTime := e.Attr("data-start-time")
		endTime := e.Attr("data-end-time")
		if startTime == "" || endTime == "" {
			return
		}

		// Extract filmweb ID from the nested ribbon div.
		filmIDStr := e.ChildAttr("div.ribbon", "data-id")
		filmID, err := strconv.Atoi(filmIDStr)
		if err != nil {
			filmID = 0
		}

		title := e.ChildText("a.tvPageGroupedSeances__title")
		if title == "" {
			return
		}

		// Extract year from the link href: /film/Title-Year-ID
		var year int
		link := e.ChildAttr("a.tvPageGroupedSeances__title", "href")
		if strings.HasPrefix(link, "/film/") {
			parts := strings.Split(strings.TrimPrefix(link, "/film/"), "-")
			if len(parts) >= 3 {
				// Year is the second-to-last segment.
				year, _ = strconv.Atoi(parts[len(parts)-2])
			}
		}

		results = append(results, epgEntry{
			title:   title,
			year:    year,
			filmID:  filmID,
			start:   startTime,
			stop:    endTime,
			isMovie: true,
		})
	})

	url := fmt.Sprintf("https://%s/program-tv/%s", filmwebURL, channelKey)
	if err := c.Visit(url); err != nil {
		return nil, fmt.Errorf("visit %s: %w", url, err)
	}

	log.Debug("scraped channel EPG", "channel", channelKey, "entries", len(results))
	return results, nil
}
