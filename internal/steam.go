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

var regex = regexp.MustCompile(`[\\\/:\*\?|"<>]*`)

func (s *Screenshot) FileName(number int) string {
	game := strings.ToLower(strings.ReplaceAll(s.Game, " ", "-"))
	date := s.Date.Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%d.webp", date, regex.ReplaceAllString(game, ""), number)
}

func MapScreenshotIDs(screenshots []Screenshot) (screenshotIDs []string) {
	for _, s := range screenshots {
		screenshotIDs = append(screenshotIDs, s.ID)
	}
	return
}

func MapScreenshotURLs(screenshots []Screenshot) (screenshotURLs []string) {
	for _, s := range screenshots {
		screenshotURLs = append(screenshotURLs, s.URL)
	}
	return
}
