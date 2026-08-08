package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var processedScreenshots []string

func InitSteam(state []string) {
	processedScreenshots = state
}

const (
	maxRequestAttempts = 4
	// The store endpoints rate limit without sending Retry-After, and a tripped
	// limit was measured lasting around 105 seconds, so the cooldown is set
	// comfortably beyond that. Requests made while limited appear to extend it,
	// which is why the whole pipeline pauses rather than each worker retrying on
	// its own schedule.
	rateLimitCooldown  = 2 * time.Minute
	minRequestInterval = 1500 * time.Millisecond
)

var (
	requestMu     sync.Mutex
	nextRequestAt time.Time
)

// httpClient carries a timeout because the default client waits forever, so a
// single stalled connection would hang the whole run with no output.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

// waitForTurn spaces requests out across every worker and honours any cooldown
// a 429 has imposed.
func waitForTurn() {
	requestMu.Lock()
	wait := time.Until(nextRequestAt)
	if wait < 0 {
		wait = 0
	}
	nextRequestAt = time.Now().Add(wait + minRequestInterval)
	requestMu.Unlock()
	if wait > 0 {
		time.Sleep(wait)
	}
}

// pauseAll pushes the next allowed request out for every worker at once.
func pauseAll(d time.Duration) {
	requestMu.Lock()
	defer requestMu.Unlock()
	if until := time.Now().Add(d); until.After(nextRequestAt) {
		nextRequestAt = until
	}
}

// getPaced fetches a rate limited endpoint, pausing every worker on a 429
// rather than hammering on regardless.
func getPaced(url string) (*http.Response, error) {
	for attempt := 1; ; attempt++ {
		waitForTurn()
		resp, err := httpClient.Get(url)
		if err != nil {
			return nil, fmt.Errorf("could not get %s: %w", url, err)
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return resp, nil
		}
		status := resp.StatusCode
		resp.Body.Close()
		if attempt == maxRequestAttempts {
			return nil, fmt.Errorf("got status %d for %s", status, url)
		}
		fmt.Printf("Got status %d for %s, pausing all requests for %s (attempt %d/%d)\n",
			status, url, rateLimitCooldown, attempt, maxRequestAttempts)
		pauseAll(rateLimitCooldown)
	}
}

func GetGameApiDetails(appId string) (GameApiDetails, error) {
	resp, err := getPaced("https://store.steampowered.com/api/appdetails?appids=" + appId)
	if err != nil {
		return GameApiDetails{}, fmt.Errorf("could not get app API details for %s: %w", appId, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GameApiDetails{}, fmt.Errorf("could not get app API details for %s: status %d", appId, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GameApiDetails{}, fmt.Errorf("could not read body for %s: %w", appId, err)
	}
	var apiResponse map[string]App
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return GameApiDetails{}, fmt.Errorf("could not unmarshal body for %s: %w", appId, err)
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
	}, nil
}

var appListResponse AppListResponse = AppListResponse{}

func GetSteamId(game string) (steamId string) {
	if len(appListResponse.AppList.Apps) == 0 {
		resp, err := httpClient.Get("http://api.steampowered.com/ISteamApps/GetAppList/v0002/?format=json")
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
