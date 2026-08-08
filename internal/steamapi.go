package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"time"
)

var steamApiKey string

func InitSteamApi(key string) {
	steamApiKey = key
}

const userFilesPageSize = 100

// screenshotFileType is what GetUserFiles expects for profile screenshots. Note
// it differs from the file_type those same screenshots report back in
// GetDetails responses, which is 5 - they are separate enums, so do not "fix"
// this to match.
const screenshotFileType = 4

// storeTagCount matches the number of tags the store page used to show.
const storeTagCount = 20

type publishedFile struct {
	PublishedFileId string `json:"publishedfileid"`
	ConsumerAppId   int    `json:"consumer_appid"`
	AppName         string `json:"app_name"`
	FileUrl         string `json:"file_url"`
	TimeCreated     int64  `json:"time_created"`
}

type userFilesResponse struct {
	Response struct {
		Total int             `json:"total"`
		Files []publishedFile `json:"publishedfiledetails"`
	} `json:"response"`
}

type vanityResponse struct {
	Response struct {
		SteamId string `json:"steamid"`
		Success int    `json:"success"`
	} `json:"response"`
}

func callSteamApi(endpoint string, params url.Values, out any) error {
	params.Set("key", steamApiKey)
	resp, err := http.Get("https://api.steampowered.com/" + endpoint + "?" + params.Encode())
	if err != nil {
		return fmt.Errorf("could not call %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not call %s: status %d", endpoint, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read %s response: %w", endpoint, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("could not unmarshal %s response: %w", endpoint, err)
	}
	return nil
}

// ResolveSteamId turns a vanity profile name into the 64 bit id the API needs.
// A numeric id is passed straight through.
func ResolveSteamId(user string) (string, error) {
	if _, err := strconv.ParseUint(user, 10, 64); err == nil {
		return user, nil
	}
	var v vanityResponse
	if err := callSteamApi("ISteamUser/ResolveVanityURL/v1/", url.Values{"vanityurl": {user}}, &v); err != nil {
		return "", err
	}
	if v.Response.Success != 1 || v.Response.SteamId == "" {
		return "", fmt.Errorf("could not resolve steam user %q", user)
	}
	return v.Response.SteamId, nil
}

type tagListResponse struct {
	Response struct {
		Tags []struct {
			TagId int    `json:"tagid"`
			Name  string `json:"name"`
		} `json:"tags"`
	} `json:"response"`
}

type storeItem struct {
	AppId     int   `json:"appid"`
	TagIds    []int `json:"tagids"`
	BasicInfo struct {
		Franchises []struct {
			Name string `json:"name"`
		} `json:"franchises"`
	} `json:"basic_info"`
}

type storeItemsResponse struct {
	Response struct {
		StoreItems []storeItem `json:"store_items"`
	} `json:"response"`
}

var tagNameCache map[int]string

// tagNames maps Steam's numeric tag ids to names. GetItems only returns ids, and
// the list is small and stable, so it is fetched once per run.
func tagNames() (map[int]string, error) {
	if tagNameCache != nil {
		return tagNameCache, nil
	}
	var r tagListResponse
	if err := callSteamApi("IStoreService/GetTagList/v1/", url.Values{"language": {"english"}}, &r); err != nil {
		return nil, err
	}
	names := make(map[int]string, len(r.Response.Tags))
	for _, t := range r.Response.Tags {
		names[t.TagId] = t.Name
	}
	tagNameCache = names
	return names, nil
}

// GetGameStoreDetails reads the tags and franchise that used to be scraped off
// the store page in a browser. The API returns the same tags in the same order,
// and unlike the store page it is not age gated.
func GetGameStoreDetails(appId string) (details GameStoreDetails, err error) {
	id, err := strconv.Atoi(appId)
	if err != nil {
		return GameStoreDetails{}, fmt.Errorf("invalid app id %q: %w", appId, err)
	}
	names, err := tagNames()
	if err != nil {
		return GameStoreDetails{}, err
	}
	request, err := json.Marshal(map[string]any{
		"ids":     []map[string]int{{"appid": id}},
		"context": map[string]string{"language": "english", "country_code": "GB"},
		"data_request": map[string]any{
			"include_basic_info": true,
			"include_tag_count":  storeTagCount,
		},
	})
	if err != nil {
		return GameStoreDetails{}, fmt.Errorf("could not build request for %s: %w", appId, err)
	}
	var r storeItemsResponse
	if err := callSteamApi("IStoreBrowseService/GetItems/v1/", url.Values{"input_json": {string(request)}}, &r); err != nil {
		return GameStoreDetails{}, err
	}
	if len(r.Response.StoreItems) == 0 {
		return GameStoreDetails{}, fmt.Errorf("no store item returned for %s", appId)
	}
	item := r.Response.StoreItems[0]
	tags := make([]string, 0, len(item.TagIds))
	for _, tagId := range item.TagIds {
		if name, ok := names[tagId]; ok {
			tags = append(tags, name)
		}
	}
	franchise := ""
	if len(item.BasicInfo.Franchises) > 0 {
		franchise = item.BasicInfo.Franchises[0].Name
	}
	return GameStoreDetails{Tags: tags, Franchise: franchise}, nil
}

// screenshotUrl rebuilds the community URL that .state is keyed on, so
// switching from scraping to the API does not reprocess everything.
func screenshotUrl(publishedFileId string) string {
	return "https://steamcommunity.com/sharedfiles/filedetails/?id=" + publishedFileId
}

func (f publishedFile) screenshot() Screenshot {
	return Screenshot{
		URL:      screenshotUrl(f.PublishedFileId),
		SteamId:  strconv.Itoa(f.ConsumerAppId),
		Game:     f.AppName,
		ImageURL: f.FileUrl,
		// time_created is the upload time, which is what the screenshot page
		// used to display as "Posted".
		Date: time.Unix(f.TimeCreated, 0).UTC(),
	}
}

// PullPublicScreenshots lists every screenshot on a profile through the Steam
// Web API and returns the ones not already in state. This replaces loading each
// screenshot page in a browser, which is what tripped Steam's rate limiting.
func PullPublicScreenshots(user string) (screenshots []Screenshot) {
	fmt.Println("Pulling public screenshots for " + user)
	steamId, err := ResolveSteamId(user)
	if err != nil {
		fmt.Printf("Could not pull screenshots: %v\n", err)
		return nil
	}

	listed := 0
	for page := 1; ; page++ {
		var r userFilesResponse
		params := url.Values{
			"steamid":    {steamId},
			"filetype":   {strconv.Itoa(screenshotFileType)},
			"numperpage": {strconv.Itoa(userFilesPageSize)},
			"page":       {strconv.Itoa(page)},
		}
		if err := callSteamApi("IPublishedFileService/GetUserFiles/v1/", params, &r); err != nil {
			fmt.Printf("Could not list screenshots: %v\n", err)
			break
		}
		if len(r.Response.Files) == 0 {
			break
		}
		for _, f := range r.Response.Files {
			listed++
			if f.FileUrl == "" {
				// Screenshots still processing have no image yet.
				continue
			}
			s := f.screenshot()
			if !slices.Contains(processedScreenshots, s.URL) {
				screenshots = append(screenshots, s)
			}
		}
		fmt.Printf("Listed %d of %d screenshots, %d to process\n", listed, r.Response.Total, len(screenshots))
		if listed >= r.Response.Total {
			break
		}
	}

	sort.Slice(screenshots, func(i, j int) bool {
		return screenshots[i].URL < screenshots[j].URL
	})
	fmt.Printf("Found %d screenshots to process in total\n", len(screenshots))
	return
}
