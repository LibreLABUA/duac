package main

import (
	"fmt"

	"github.com/gocolly/colly"
)

func login(user, pass string) (colly.Collector, error) {
	c := colly.Collector{
		UserAgent:   "Pakillo 4.20",
		MaxBodySize: 100 * 1024 * 1024,
	}
	c.Init()

	exec := ""
	c.OnHTML("input[value]", func(e *colly.HTMLElement) {
		if s := e.Attr("value"); e.Attr("name") == "execution" && s != "" {
			exec = s
		}
	})
	defer c.OnHTMLDetach("input[value]")

	// Be patient... UA's webpage is written in C#
	err := c.Visit(urlNormal)
	if err != nil {
		return c, err
	}
	if exec == "" {
		return c, fmt.Errorf("error getting UA login values")
	}

	err = c.Post(urlLogin, map[string]string{
		"_eventId":    "submit",
		"username":    user,
		"password":    pass,
		"geolocation": "",
		"execution":   exec,
	})
	if err != nil {
		return c, err
	}

	return c, err
}
