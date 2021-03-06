package main

import (
	"github.com/gocolly/colly"
	"strconv"
	"strings"
	"time"
)

type UserAgent string

const (
	Chrome  UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.246"
	Firefox UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:62.0) Gecko/20100101 Firefox/62.0"
)

type JobType string

const (
	SellJob JobType = "sell"
	RentJob JobType = "hand_over"
)

type JobLang string

const (
	Ru JobLang = "ru"
	Lv JobLang = "lv"
)

type City string

const RigaCity = City("Riga")

type Interval string

const (
	All    Interval = "all"
	Today  Interval = "today"
	Today2 Interval = "today-2"
	Today5 Interval = "today-5"
)

const BaseUrl = "https://www.ss.lv/"

type Command struct {
	UserAgent UserAgent
	JobType   JobType
	Lang      JobLang
	City      City
	Interval  Interval
}

func (c *Command) ConstructUrl() string {
	return BaseUrl +
		string(c.Lang) +
		"/real-estate/flats/" +
		string(c.City) +
		"/" +
		string(c.Interval) +
		"/" +
		string(c.JobType) +
		"/"
}

type Crawler struct {
	logger    Logger
	collector *colly.Collector
}

func (c *Crawler) RunCommand(command Command) FlatStorage {
	flatStorage := NewFlatStorage()

	c.collector = colly.NewCollector(
		colly.UserAgent(string(command.UserAgent)),
		colly.AllowedDomains("www.ss.lv"),
	)

	c.collector.Limit(&colly.LimitRule{
		DomainGlob:  "*ss.lv*",
		Parallelism: 1,
		RandomDelay: 1 * time.Second,
	})

	c.collector.OnHTML("tr[id]", func(rowElement *colly.HTMLElement) {
		idStr := rowElement.Attr("id")
		if !strings.HasPrefix(idStr, "tr_") {
			return
		}
		if strings.HasPrefix(idStr, "tr_bnr") {
			return
		}
		flat := Flat{}
		flat.City = string(command.City)
		flat.Id, _ = strconv.Atoi(rowElement.Attr("id")[3:])
		// ugly fix :)
		if command.JobType == RentJob {
			flat.Type = "rent"
		} else {
			flat.Type = string(command.JobType)
		}
		rowElement.ForEach("td", func(i int, cellElement *colly.HTMLElement) {
			switch i {
			case 2:
				flat.Text = FilterChars(cellElement.Text, "[\n]")
				cellElement.ForEach("a[href]", func(i int, element *colly.HTMLElement) {
					flat.Url = element.Request.AbsoluteURL(element.Attr("href"))
				})
			case 3:
				locationText, _ := cellElement.DOM.Html()
				locationText = strings.ReplaceAll(locationText, "<b>", "")
				locationText = strings.ReplaceAll(locationText, "</b>", "")
				locationArr := strings.Split(locationText, "<br/>")
				// street is not actual for some cities
				if len(locationArr) == 2 {
					flat.District, flat.Street = locationArr[0], locationArr[1]
				} else {
					flat.District = locationText
				}
			case 4:
				flat.Rooms, _ = strconv.Atoi(cellElement.Text)
			case 5:
				flat.ApartmentArea, _ = strconv.Atoi(cellElement.Text)
			case 6:
				flat.Floor = cellElement.Text
			case 7:
				flat.HouseType = cellElement.Text
			case 8:
				flat.Price, _ = strconv.Atoi(FilterChars(cellElement.Text, "[^0-9]"))
			case 9:
				flat.Price, _ = strconv.Atoi(FilterChars(cellElement.Text, "[^0-9]"))
			}
		})
		flatStorage.Put(flat)
	})

	visitedUrls := make([]string, 50)

	c.collector.OnHTML("a[name]", func(element *colly.HTMLElement) {
		url := element.Request.AbsoluteURL(element.Attr("href"))
		if !IsStringInSlice(visitedUrls, url) {
			visitedUrls = append(visitedUrls, url)
			c.logger.Log("Visit " + url)
			if err := c.collector.Visit(url); err != nil {
				c.logger.Log(err.Error())
			}
		}
	})

	url := command.ConstructUrl()
	visitedUrls = append(visitedUrls, url)

	c.logger.Log("Initial url: " + url)

	if err := c.collector.Visit(url); err != nil {
		c.logger.Log(err.Error())
	}

	return flatStorage
}
