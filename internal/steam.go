package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
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
	maxConcurrent := 3
	guard := make(chan struct{}, maxConcurrent)
	hrefs := getScreenshotsToProcess(user)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < len(hrefs); i++ {
		guard <- struct{}{}
		wg.Add(1)
		go func() {
			fmt.Println("Pulling screenshot " + hrefs[i])
			defer wg.Done()
			defer func() { <-guard }()
			screenshot, err := PullScreenshot(hrefs[i])
			if err != nil {
				fmt.Printf("Skipping %s: %v\n", hrefs[i], err)
				return
			}
			fmt.Println("Processing: " + screenshot.Date.Format("2006 Jan 02") + " - " + screenshot.Game)
			mu.Lock()
			screenshots = append(screenshots, screenshot)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].URL < screenshots[j].URL
	})
	return
}

const maxNavAttempts = 4

// Steam rate limits when pages are pulled in parallel. Goto only returns an
// error for network failures, so the status has to be checked by hand or a 429
// looks like a successful navigation and every locator on the page times out.
func gotoWithRetry(page playwright.Page, url string) error {
	backoff := time.Second
	for attempt := 1; ; attempt++ {
		response, err := page.Goto(url)
		if err != nil {
			return fmt.Errorf("could not goto %s: %w", url, err)
		}
		if response == nil || response.Status() == 200 {
			return nil
		}
		status := response.Status()
		if attempt == maxNavAttempts || !(status == 429 || status >= 500) {
			return fmt.Errorf("got status %d for %s", status, url)
		}
		fmt.Printf("Got status %d for %s, retrying in %s\n", status, url, backoff)
		time.Sleep(backoff)
		backoff *= 2
	}
}

func getScreenshotsToProcess(user string) (screenshotUrls []string) {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	url := fmt.Sprintf("https://steamcommunity.com/id/%s/screenshots/", user)
	if err := gotoWithRetry(page, url); err != nil {
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

func PullScreenshot(url string) (screenshot Screenshot, err error) {
	page, err := browser.NewPage()
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not create page: %w", err)
	}
	defer page.Close()
	if err := gotoWithRetry(page, url); err != nil {
		return Screenshot{}, err
	}
	imageUrl, err := page.Locator(".actualmediactn > a").First().GetAttribute("href")
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not get image href: %w", err)
	}
	storeUrl, err := page.Locator(".breadcrumbs > a").First().GetAttribute("href")
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not get store href: %w", err)
	}
	stats, err := page.Locator(".detailsStatRight").All()
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not get stats: %w", err)
	}
	if len(stats) < 2 {
		return Screenshot{}, fmt.Errorf("expected at least 2 stats, got %d", len(stats))
	}
	postedTime, err := stats[1].TextContent()
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not get stats: %w", err)
	}
	postedDate := strings.Split(postedTime, " @")[0]
	actualDate, err := ParseTime(postedDate)
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not parse date: %w", err)
	}
	appName, err := page.Locator(".apphub_AppName").First().TextContent()
	if err != nil {
		return Screenshot{}, fmt.Errorf("could not get app name: %w", err)
	}
	re := regexp.MustCompile(".*/app/(\\d*).*")
	appMatch := re.FindStringSubmatch(storeUrl)
	if appMatch == nil {
		return Screenshot{}, fmt.Errorf("could not find app id in %s", storeUrl)
	}
	return Screenshot{URL: url, Game: appName, ImageURL: imageUrl, Date: actualDate, SteamId: appMatch[1]}, nil
}

func GetGameApiDetails(appId string) GameApiDetails {
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
	return GameApiDetails{
		Name:       apiResponse[appId].Data.Name,
		Genres:     genres,
		Price:      apiResponse[appId].Data.PriceOverview.Initial,
		Developers: apiResponse[appId].Data.Developers,
		Publishers: apiResponse[appId].Data.Publishers,
	}
}

func GetGameStoreDetails(appId string) (details GameStoreDetails, err error) {
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	defer page.Close()
	if err := gotoWithRetry(page, "https://store.steampowered.com/app/"+appId); err != nil {
		return GameStoreDetails{}, errors.New("Could not go to store page for " + appId)
	}
	tagLinks, err := page.Locator(".app_tag").All()
	if err != nil {
		log.Fatalf("could not get dev links: %v", err)
	}
	tags := []string{}
	for _, link := range tagLinks {
		tag, _ := link.InnerText()
		if strings.Contains(tag, "+") {
			continue
		}
		tags = append(tags, strings.TrimSpace(tag))
	}
	child := page.GetByText("Franchise")
	franchiseContainer := page.Locator(".dev_row").Filter(playwright.LocatorFilterOptions{Has: child})
	franchise, _ := franchiseContainer.Locator("a").TextContent()
	return GameStoreDetails{Tags: tags, Franchise: franchise}, nil
}

var appListResponse AppListResponse = AppListResponse{}

func GetSteamId(game string) (steamId string) {
	if len(appListResponse.AppList.Apps) == 0 {
		resp, err := http.Get("http://api.steampowered.com/ISteamApps/GetAppList/v0002/?format=json")
		if err != nil {
			log.Fatalf("could not get app API details: %v", err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("could not read body: %v", err)
		}
		err = json.Unmarshal(body, &appListResponse)
		if err != nil {
			log.Fatalf("could not unmarshal body: %v", err)
		}
	}
	for _, a := range appListResponse.AppList.Apps {
		if a.Name == game {
			steamId = strconv.Itoa(a.AppId)
			break
		}
	}
	if steamId == "" {
		fmt.Println("Could not find Steam ID for " + game + ". Please add Steam ID manually")
	}
	return
}

type AppListResponse struct {
	AppList AppList `json:"applist"`
}

type AppList struct {
	Apps []ListedApp `json:"apps"`
}

type ListedApp struct {
	AppId int    `json:"appid"`
	Name  string `json:"name"`
}

type GameApiDetails struct {
	Name       string
	Genres     []string
	Price      int
	Developers []string `json:"developers"`
	Publishers []string `json:"publishers"`
}

type GameStoreDetails struct {
	Tags      []string
	Franchise string
}

type App struct {
	Data Data `json:"data"`
}

type Data struct {
	Genres        []Genre       `json:"genres"`
	Name          string        `json:"name"`
	Developers    []string      `json:"developers"`
	Publishers    []string      `json:"publishers"`
	PriceOverview PriceOverview `json:"price_overview"`
}

type Genre struct {
	Description string `json:"description"`
}

type PriceOverview struct {
	Initial int `json:"initial"`
}

type Screenshot struct {
	URL      string
	SteamId  string
	Game     string
	ImageURL string
	Date     time.Time
}

var regex = regexp.MustCompile(`[\\\/:\*\?|"<>]*`)

func (s *Screenshot) FileName(number int) string {
	game := strings.ToLower(strings.ReplaceAll(s.Game, " ", "-"))
	date := s.Date.Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%d.webp", date, regex.ReplaceAllString(game, ""), number)
}
