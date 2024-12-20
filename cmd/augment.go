package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/development-and-dinosaurs/paleoplay/internal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var augmentCmd = &cobra.Command{
	Use:   "augment",
	Short: "Augment existing blog posts with additional details.",
	Long: `Augment existing blog posts with additional details.
	
This will work through the list of blog posts and add any additional 
details that are missing, such as those added in later versions of
Paleoplay.`,
	Run: func(cmd *cobra.Command, args []string) {
		posts, err := os.ReadDir("content/posts")
		if err != nil {
			log.Fatalf("could not get posts: %v", err)
		}
		internal.InitSteam([]string{})
		for _, post := range posts {
			p := internal.PostData{}
			contents, err := os.ReadFile("content/posts/" + post.Name())
			if err != nil {
				fmt.Println(err)
			}
			yaml.Unmarshal(contents, &p)
			if p.SteamId == "" {
				p.SteamId = internal.GetSteamId(p.Game)
			}
			if p.SteamId != "" && p.Paleoplay < internal.PaleoplayVersion {
				fmt.Println("Augmenting post " + p.FileName())
				internal.AugmentPost(p)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(augmentCmd)
}
