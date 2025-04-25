package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gocolly/colly"
)

const FILMWEB_URL = "www.filmweb.pl"

func get_channel_epg(channel string) []EpgEntry {
	epg_entries := []EpgEntry{}
	results := []EpgEntry{}
	c := colly.NewCollector(colly.AllowedDomains(FILMWEB_URL))

	c.OnHTML("div", func(e *colly.HTMLElement) {
		if e.Attr("data-sid") != "" {
			film_id, err := strconv.Atoi(e.Attr("data-film"))
			if err != nil {
				film_id = 0
			}
			sid, err := strconv.Atoi(e.Attr("data-sid"))
			if err != nil {
				sid = 0
			}

			link := e.ChildAttr("a", "href")
			var year int
			if strings.HasPrefix(link, "/film/") {
				link_slice := strings.Split(link, "/")
				title_year_id := strings.Split(link_slice[2], "-")
				year, err = strconv.Atoi(title_year_id[1])
				if err != nil {
					year = 0
				}
			}
			new_entry := EpgEntry{
				e.ChildText("a"),
				year,
				film_id,
				sid,
				e.Attr("data-start"),
				"",
				strings.Contains(e.Attr("class"), "film"),
			}
			epg_entries = append(epg_entries, new_entry)
		}
	})

	c.Visit(fmt.Sprintf("https://%s/program-tv/%s", FILMWEB_URL, channel))

	for i := 0; i < len(epg_entries); i++ {
		if i < len(epg_entries)-1 {
			epg_entries[i].stop = epg_entries[i+1].start
		}
		if epg_entries[i].is_movie && epg_entries[i].title != "" {
			results = append(results, epg_entries[i])
		}
	}
	return results
}
