package internal

import (
	"fmt"
	"os"
	"strings"
	"text/template"
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
}

func CreatePost(group ImageGrouping) {
	apiDetails := GetGameApiDetails(group.SteamId)
	storeDetails, err := GetGameStoreDetails(group.SteamId)
	if err != nil {
		fmt.Println(err)
		return
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
	writePost(postData)
}

var gameApiDetails = make(map[string]GameApiDetails)
var gameStoreDetails = make(map[string]GameStoreDetails)

func AugmentPost(postData PostData) {
	steamId := postData.SteamId
	apiDetails, inCache := gameApiDetails[steamId]
	if !inCache {
		apiDetails = GetGameApiDetails(steamId)
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
	writePost(postData)
}

func writePost(postData PostData) {
	tmpl, err := template.New("post").Parse(postTemplate)
	if err != nil {
		panic(err)
	}
	filename := "content/posts/" + postData.FileName()
	os.MkdirAll("content/posts/", 0777)
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(file, postData)
	if err != nil {
		panic(err)
	}
}

func GroupScreenshots(screenshots []Screenshot) (imageGroupings []ImageGrouping) {
	gameOnDate := GroupByProperty(screenshots, func(s Screenshot) string { return s.Date.Format("2006-01-02") + s.Game })
	for _, v := range gameOnDate {
		images := make([]Image, len(v))
		for i := range v {
			images[i] = Image{v[i].ImageURL, v[i].FileName(i + 1)}
		}
		imageGrouping := ImageGrouping{
			SteamId: v[0].SteamId,
			Date:    v[0].Date.Format("2006-01-02"),
			Images:  images,
		}
		imageGroupings = append(imageGroupings, imageGrouping)
	}
	return
}
