package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

var pw *playwright.Playwright
var browser playwright.Browser
var page playwright.Page
var processedScreenshots []string

func InitSteam(state []string) {
	processedScreenshots = state
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	browser, err = pw.Chromium.Launch()
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	page, err = browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
}

func CloseSteam() {
	if err := browser.Close(); err != nil {
		log.Fatalf("could not close browser: %v", err)
	}
}

func PullPublicScreenshots(user string) (screenshots []Screenshot) {
	url := fmt.Sprintf("https://steamcommunity.com/id/%s/screenshots/", user)
	if _, err := page.Goto(url); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	entries, err := page.Locator(".profile_media_item").All()
	if err != nil {
		log.Fatalf("could not get entries: %v", err)
	}
	hrefs := make([]string, 0)
	for _, entry := range entries {
		href, _ := entry.GetAttribute("href")
		hrefs = append(hrefs, href)
	}
	for _, href := range hrefs {
		if !slices.Contains(processedScreenshots, href) {
			screenshot := PullScreenshot(href)
			fmt.Println(screenshot.Date.Format("2006 Jan 02") + " - " + screenshot.Game)
			screenshots = append(screenshots, screenshot)
		}
	}
	return
}

func PullScreenshot(url string) (screenshot Screenshot) {
	if _, err := page.Goto(url); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	imageUrl, err := page.Locator(".actualmediactn > a").First().GetAttribute("href")
	if err != nil {
		log.Fatalf("could not get image href: %v", err)
	}
	storeUrl, err := page.Locator(".apphub_HeaderTop > div > a").First().GetAttribute("href")
	if err != nil {
		log.Fatalf("could not get store href: %v", err)
	}
	stats, err := page.Locator(".detailsStatRight").All()
	if err != nil {
		log.Fatalf("could not get stats: %v", err)
	}
	postedTime, err := stats[1].TextContent()
	if err != nil {
		log.Fatalf("could not get stats: %v", err)
	}
	postedDate := strings.Split(postedTime, " @")[0]
	actualDate, err := ParseTime(postedDate)
	if err != nil {
		log.Fatalf("could not parse date: %v", err)
	}
	re := regexp.MustCompile(".*/app/(\\d*).*")
	appId := re.FindStringSubmatch(storeUrl)[1]
	details := getGameDetails(appId)
	return Screenshot{ID: url, Game: details.Name, URL: imageUrl, Date: actualDate, Genres: details.Genres}
}

func getGameDetails(appId string) GameDetails {
	resp, err := http.Get("https://store.steampowered.com/api/appdetails?appids=" + appId)
	if err != nil {
		log.Fatalf("could not get app API details: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("could not read body: %v", err)
	}
	var apiResponse map[string]App
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Fatalf("could not unmarshal body: %v", err)
	}
	genres := make([]string, 0)
	for _, g := range apiResponse[appId].Data.Genres {
		genres = append(genres, g.Description)
	}
	return GameDetails{Name: apiResponse[appId].Data.Name, Genres: genres}
}

type GameDetails struct {
	Name   string
	Genres []string
}

type App struct {
	Data Data `json:"data"`
}

type Data struct {
	Genres []Genre `json:"genres"`
	Name   string  `json:"name"`
}

type Genre struct {
	Description string `json:"description"`
}

type Screenshot struct {
	ID     string
	Game   string
	URL    string
	Date   time.Time
	Genres []string
}

func (s *Screenshot) FileName() string {
	game := strings.ToLower(strings.ReplaceAll(s.Game, " ", "-"))
	date := s.Date.Format("2006-01-02")
	return fmt.Sprintf("%s-%s.webp", date, game)
}

func MapScreenshotIDs(screenshots []Screenshot) (screenshotURLs []string) {
	for _, s := range screenshots {
		screenshotURLs = append(screenshotURLs, s.ID)
	}
	return
}
