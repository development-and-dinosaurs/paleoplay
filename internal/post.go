package internal

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

type PostData struct {
	Date      string   `yaml:"dates"`
	Images    []string `yaml:"images"`
	Game      string   `yaml:"games"`
	Genres    []string `yaml:"genres"`
	SteamId   string   `yaml:"steamId"`
	Paleoplay int      `yaml:"paleoplay"`
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
	gameDetails := GetGameDetails(group.SteamId)
	postData := PostData{
		SteamId:   group.SteamId,
		Date:      group.Date,
		Images:    Map(group.Images, func(i Image) string { return i.Destination }),
		Game:      gameDetails.Name,
		Genres:    gameDetails.Genres,
		Paleoplay: 1,
	}
	tmpl, err := template.New("post.tmpl").ParseFiles("post.tmpl")
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
