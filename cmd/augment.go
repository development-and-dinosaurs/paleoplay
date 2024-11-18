package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/development-and-dinosaurs/paleoplay/internal"
	"github.com/spf13/cobra"
)

var regex = regexp.MustCompile(`[\\\/:\*\?|"<>]*`)

type Post struct {
	Title      string   `yaml:"title"`
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

func (p *Post) FileName() string {
	game := strings.ToLower(strings.ReplaceAll(p.Game, " ", "-"))
	date := p.Date
	return fmt.Sprintf("%s-%s.md", date, regex.ReplaceAllString(game, ""))
}

var postTemplate = `---
title: |-
  %[1]s - %[2]s
dates: %[1]s
images: 
%[3]s
games: |-
   %[2]s
price: %.2[7]f
genres:
%[4]s
developers:
%[5]s
publishers:
%[6]s
franchises: %[8]s
tags:
%[9]s
steamId: %[10]s
paleoplay: %[11]s
---
`

var augmentCmd = &cobra.Command{
	Use:   "augment",
	Short: "Augment existing blog posts with additional details.",
	Long: `Augment existing blog posts with additional details.
	
This will work through the list of blog posts and add any additional 
details that are missing, such as those added in later versions of
Paleoplay.`,
	Run: func(cmd *cobra.Command, args []string) {
		internal.CreatePost(internal.ImageGrouping{SteamId: "2213190", Date: "2022-06-01", Images: []internal.Image{{Destination: "sada"}}})
		// posts, err := os.ReadDir("content/posts")
		// if err != nil {
		// 	log.Fatalf("could not get posts: %v", err)
		// }
		// for _, post := range posts {
		// 	p := Post{}
		// 	contents, err := os.ReadFile("content/posts/" + post.Name())
		// 	if err != nil {
		// 		fmt.Println(err)
		// 	}
		// 	yaml.Unmarshal(contents, &p)
		// 	if p.SteamId == "" {
		// 		continue
		// 	}
		// 	internal.InitSteam([]string{})
		// 	apiDetails := internal.GetGameApiDetails(p.SteamId)
		// 	storeDetails := internal.GetGameStoreDetails(p.SteamId)
		// 	p.Franchise = storeDetails.Franchise
		// 	p.Price = float64(apiDetails.Price) / 100
		// 	p.Developers = storeDetails.Developers
		// 	p.Publishers = storeDetails.Publishers
		// 	p.Tags = storeDetails.Tags[:10]
		// 	CreatePost(p)
		// }
	},
}

func CreatePost(p Post) {
	genres := internal.Map(p.Genres, func(s string) string { return "  - " + s })
	images := internal.Map(p.Images, func(i string) string { return "  - images/posts/" + i })
	developers := internal.Map(p.Developers, func(s string) string { return "  - " + s })
	publishers := internal.Map(p.Publishers, func(s string) string { return "  - " + s })
	tags := internal.Map(p.Tags, func(s string) string { return "  - " + s })
	post := fmt.Sprintf(postTemplate, p.Date, p.Game, strings.Join(images, "\n"), strings.Join(genres, "\n"), strings.Join(developers, "\n"), strings.Join(publishers, "\n"), p.Price, p.Franchise, strings.Join(tags, "\n"), p.SteamId, 2)
	os.MkdirAll("content/posts/", 0777)
	os.WriteFile("content/posts/"+p.FileName(), []byte(post), 0777)
}

func init() {
	rootCmd.AddCommand(augmentCmd)
}
