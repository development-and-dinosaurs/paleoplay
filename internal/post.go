package internal

import (
	"fmt"
	"os"
	"strings"
)

var postTemplate = `---
title: |-
  %[1]s - %[2]s
dates: %[1]s
images: 
%[3]s
games: |-
   %[2]s
genres:
%[4]s
---
`

func CreatePost(p Post) {
	genres := Map(p.Genres, func(s string) string { return "  - " + s })
	images := Map(p.Images, func(i Image) string { return "  - " + i.Destination })
	post := fmt.Sprintf(postTemplate, p.Date, p.Game, strings.Join(images, "\n"), strings.Join(genres, "\n"))
	os.MkdirAll("content/posts/", 0777)
	os.WriteFile("content/posts/"+p.FileName(), []byte(post), 0777)
}

func MapPostsFromScreenshots(screenshots []Screenshot) (posts []Post) {
	gameOnDate := GroupByProperty(screenshots, func(s Screenshot) string { return s.Date.Format("2006-01-02") + s.Game })
	for _, v := range gameOnDate {
		images := make([]Image, len(v))
		for i := range v {
			images[i] = Image{v[i].URL, v[i].FileName(i + 1)}
		}
		post := Post{
			Title:  v[0].Date.Format("2006-01-02") + " - " + v[0].Game,
			Date:   v[0].Date.Format("2006-01-02"),
			Images: images,
			Game:   v[0].Game,
			Genres: v[0].Genres,
		}
		posts = append(posts, post)
	}
	return
}

type Post struct {
	Title  string
	Date   string
	Images []Image
	Game   string
	Genres []string
}

func (p *Post) FileName() string {
	game := strings.ToLower(strings.ReplaceAll(p.Game, " ", "-"))
	date := p.Date
	return fmt.Sprintf("%s-%s.md", date, regex.ReplaceAllString(game, ""))
}

type Image struct {
	Source      string
	Destination string
}
