package internal

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
)

var PaleoplayVersion = 2

type PostData struct {
	Date       string   `yaml:"dates"`
	Images     []string `yaml:"images"`
	Game       string   `yaml:"games"`
	Price      float64  `yaml:"price"`
	Genres     []string `yaml:"genres"`
	Developers []string `yaml:"developers"`
	Publishers []string `yaml:"publishers"`
	Franchise  string   `yaml:"franchises"`
	Tags       []string `yaml:"tags"`
	SteamId    string   `yaml:"steamId"`
	Paleoplay  int      `yaml:"paleoplay"`
}

func (p *PostData) FileName() string {
	game := strings.ToLower(strings.ReplaceAll(p.Game, " ", "-"))
	date := p.Date
	return fmt.Sprintf("%s-%s.md", date, regex.ReplaceAllString(game, ""))
}

type ImageGrouping struct {
	SteamId string
	Date    string
	Images  []Image
}

type Image struct {
	Source      string
	Destination string
	// Screenshot is the page the image came from, so a published post can be
	// recorded in state without re-deriving it.
	Screenshot string
}

func CreatePost(group ImageGrouping) error {
	fmt.Printf("  Fetching game details for %s\n", group.SteamId)
	apiDetails, err := GetGameApiDetails(group.SteamId)
	if err != nil {
		return err
	}
	fmt.Printf("  Fetching store details for %s\n", group.SteamId)
	storeDetails, err := GetGameStoreDetails(group.SteamId)
	if err != nil {
		return err
	}
	postData := PostData{
		SteamId:    group.SteamId,
		Date:       group.Date,
		Images:     Map(group.Images, func(i Image) string { return "images/posts/" + i.Destination }),
		Game:       apiDetails.Name,
		Franchise:  storeDetails.Franchise,
		Price:      float64(apiDetails.Price) / 100,
		Genres:     apiDetails.Genres,
		Developers: apiDetails.Developers,
		Publishers: apiDetails.Publishers,
		Tags:       storeDetails.Tags,
		Paleoplay:  PaleoplayVersion,
	}
	return writePost(postData)
}

var gameApiDetails = make(map[string]GameApiDetails)
var gameStoreDetails = make(map[string]GameStoreDetails)

func AugmentPost(postData PostData) {
	steamId := postData.SteamId
	apiDetails, inCache := gameApiDetails[steamId]
	if !inCache {
		var err error
		apiDetails, err = GetGameApiDetails(steamId)
		if err != nil {
			fmt.Println(err)
			return
		}
		gameApiDetails[steamId] = apiDetails
	}
	storeDetails, inCache := gameStoreDetails[steamId]
	if !inCache {
		var err error
		storeDetails, err = GetGameStoreDetails(steamId)
		gameStoreDetails[steamId] = storeDetails
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	postData.Price = float64(apiDetails.Price) / 100
	postData.Franchise = storeDetails.Franchise
	postData.Developers = apiDetails.Developers
	postData.Publishers = apiDetails.Publishers
	postData.Tags = storeDetails.Tags
	postData.Paleoplay = PaleoplayVersion
	if err := writePost(postData); err != nil {
		fmt.Println(err)
	}
}

func writePost(postData PostData) error {
	tmpl, err := template.New("post").Parse(postTemplate)
	if err != nil {
		return fmt.Errorf("could not parse post template: %w", err)
	}
	filename := "content/posts/" + postData.FileName()
	if err := os.MkdirAll("content/posts/", 0777); err != nil {
		return fmt.Errorf("could not create posts directory: %w", err)
	}
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("could not create %s: %w", filename, err)
	}
	defer file.Close()
	if err := tmpl.Execute(file, postData); err != nil {
		return fmt.Errorf("could not write %s: %w", filename, err)
	}
	fmt.Printf("  Wrote %s\n", filename)
	return nil
}

func GroupScreenshots(screenshots []Screenshot) (imageGroupings []ImageGrouping) {
	gameOnDate := GroupByProperty(screenshots, func(s Screenshot) string { return s.Date.Format("2006-01-02") + s.Game })
	for _, v := range gameOnDate {
		images := make([]Image, len(v))
		for i := range v {
			images[i] = Image{Source: v[i].ImageURL, Destination: v[i].FileName(i + 1), Screenshot: v[i].URL}
		}
		imageGrouping := ImageGrouping{
			SteamId: v[0].SteamId,
			Date:    v[0].Date.Format("2006-01-02"),
			Images:  images,
		}
		imageGroupings = append(imageGroupings, imageGrouping)
	}
	// Map iteration order is random, so sort to keep runs reproducible and to
	// make the cap below take the oldest posts first.
	sort.Slice(imageGroupings, func(i, j int) bool {
		if imageGroupings[i].Date != imageGroupings[j].Date {
			return imageGroupings[i].Date < imageGroupings[j].Date
		}
		return imageGroupings[i].SteamId < imageGroupings[j].SteamId
	})
	return
}

// DeferRecentGroups holds back posts dated today. More screenshots can still be
// uploaded before the day is out, and they would arrive after the post was
// written: writePost truncates by filename and image names restart at 1, so the
// later run would replace the post and clobber its images rather than add to
// them. Once a day has passed no new screenshot can carry that date, so
// deferring only the current day is enough.
func DeferRecentGroups(groups []ImageGrouping, now time.Time) []ImageGrouping {
	today := now.UTC().Format("2006-01-02")
	kept := make([]ImageGrouping, 0, len(groups))
	for _, group := range groups {
		if group.Date >= today {
			fmt.Printf("Deferring post for %s on %s until the day is complete\n", group.SteamId, group.Date)
			continue
		}
		kept = append(kept, group)
	}
	return kept
}

// CapGroups trims whole posts off the end until the run would compress no more
// than max images, guarding against a state mismatch turning into a thousand
// Tinify calls. Groups are never split: a partially published post would be
// rewritten with only its remaining images next run, because the filename is
// derived from the date and game rather than the images.
func CapGroups(groups []ImageGrouping, max int) []ImageGrouping {
	if max <= 0 {
		return groups
	}
	images := 0
	for i, group := range groups {
		// Always allow the first post through, even if it alone exceeds the
		// cap, or an oversized post would stall every future run.
		if i > 0 && images+len(group.Images) > max {
			fmt.Printf("Capping run at %d posts (%d images); %d posts left for next run\n",
				i, images, len(groups)-i)
			return groups[:i]
		}
		images += len(group.Images)
	}
	return groups
}
