package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

var pw *playwright.Playwright
var browser playwright.Browser
var processedScreenshots []string

func InitSteam(state []string) {
	processedScreenshots = state
	err := playwright.Install(&playwright.RunOptions{Browsers: []string{"chromium"}})
	if err != nil {
		log.Fatalf("could not install playwright: %v", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	browser, err = pw.Chromium.Launch()
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
}

func CloseSteam() {
	if err := browser.Close(); err != nil {
		log.Fatalf("could not close browser: %v", err)
	}
}

func PullPublicScreenshots(user string) (screenshots []Screenshot) {
	fmt.Println("Pulling public screenshots for " + user)
	maxConcurrent := 10
	guard := make(chan struct{}, maxConcurrent)
	hrefs := getScreenshotsToProcess(user)
	var wg sync.WaitGroup
	for i := 0; i < len(hrefs); i++ {
		guard <- struct{}{}
		wg.Add(1)
		go func() {
			fmt.Println("Pulling screenshot " + hrefs[i])
			defer wg.Done()
			screenshot := PullScreenshot(hrefs[i])
			fmt.Println("Processing: " + screenshot.Date.Format("2006 Jan 02") + " - " + screenshot.Game)
			screenshots = append(screenshots, screenshot)
			<-guard
		}()
	}
	wg.Wait()
	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].ID < screenshots[j].ID
	})
	return
}

func getScreenshotsToProcess(user string) (screenshotUrls []string) {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	url := fmt.Sprintf("https://steamcommunity.com/id/%s/screenshots/", user)
	if _, err := page.Goto(url); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	stillProcessing := true
	for stillProcessing {
		items, _ := page.Locator(".profile_media_item").All()
		toProcess := items[len(items)-12:]
		for _, item := range toProcess {
			href, _ := item.GetAttribute("href")
			if !slices.Contains(processedScreenshots, href) {
				screenshotUrls = append(screenshotUrls, href)
			} else {
				fmt.Printf("Found %d screenshots to process in total\n", len(screenshotUrls))
				return
			}
		}
		fmt.Printf("Found %d screenshots to process, looking for more...\n", len(items))
		page.Mouse().Wheel(0, 15000)
		time.Sleep(time.Millisecond * 500)
		reachedBottom, _ := page.Locator("#EndOfInfiniteContent").Count()
		stillProcessing = reachedBottom != 1
	}
	return
}

func PullScreenshot(url string) (screenshot Screenshot) {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	if _, err := page.Goto(url); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	imageUrl, err := page.Locator(".actualmediactn > a").First().GetAttribute("href")
	if err != nil {
		log.Fatalf("could not get image href: %v", err)
	}
	storeUrl, err := page.Locator(".breadcrumbs > a").First().GetAttribute("href")
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
	appName, err := page.Locator(".apphub_AppName").First().TextContent()
	if err != nil {
		log.Fatalf("could not get app name: %v", err)
	}
	re := regexp.MustCompile(".*/app/(\\d*).*")
	appId := re.FindStringSubmatch(storeUrl)[1]
	return Screenshot{ID: url, Game: appName, URL: imageUrl, Date: actualDate, SteamId: appId}
}

func GetGameApiDetails(appId string) GameApiDetails {
	resp, err := http.Get("https://store.steampowered.com/api/appdetails?cc=GB&appids=" + appId)
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
	return GameApiDetails{Name: apiResponse[appId].Data.Name, Genres: genres, Price: apiResponse[appId].Data.PriceOverview.Initial}
}

func GetGameStoreDetails(appId string) GameStoreDetails {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	if _, err := page.Goto("https://store.steampowered.com/app/" + appId); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	devLinks, err := page.Locator(".dev_row a").All()
	if err != nil {
		log.Fatalf("could not get dev links: %v", err)
	}
	developers := []string{}
	publishers := []string{}
	for _, link := range devLinks {
		href, _ := link.GetAttribute("href")
		if strings.Contains(href, "/developer/") {
			developer, _ := link.TextContent()
			developers = append(developers, developer)
		}
		if strings.Contains(href, "/publisher/") {
			publisher, _ := link.TextContent()
			publishers = append(publishers, publisher)
		}
	}
	slices.Sort(developers)
	slices.Sort(publishers)
	developers = slices.Compact(developers)
	publishers = slices.Compact(publishers)
	tagLinks, err := page.Locator(".app_tag").All()
	if err != nil {
		log.Fatalf("could not get dev links: %v", err)
	}
	tags := []string{}
	for _, link := range tagLinks {
		tag, _ := link.InnerText()
		tags = append(tags, strings.TrimSpace(tag))
	}
	child := page.GetByText("Franchise")
	franchiseContainer := page.Locator(".dev_row").Filter(playwright.LocatorFilterOptions{Has: child})
	franchise, _ := franchiseContainer.Locator("a").TextContent()
	return GameStoreDetails{Developers: developers, Publishers: publishers, Tags: tags, Franchise: franchise}
}

type GameApiDetails struct {
	Name   string
	Genres []string
	Price  int
}

type GameStoreDetails struct {
	Developers []string
	Publishers []string
	Tags       []string
	Franchise  string
}

type App struct {
	Data Data `json:"data"`
}

type Data struct {
	Genres        []Genre       `json:"genres"`
	Name          string        `json:"name"`
	PriceOverview PriceOverview `json:"price_overview"`
}

type Genre struct {
	Description string `json:"description"`
}

type PriceOverview struct {
	Initial int `json:"initial"`
}

type Screenshot struct {
	SteamId string
	ID      string
	Game    string
	URL     string
	Date    time.Time
	Genres  []string
}

var regex = regexp.MustCompile(`[\\\/:\*\?|"<>]*`)

func (s *Screenshot) FileName(number int) string {
	game := strings.ToLower(strings.ReplaceAll(s.Game, " ", "-"))
	date := s.Date.Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%d.webp", date, regex.ReplaceAllString(game, ""), number)
}
