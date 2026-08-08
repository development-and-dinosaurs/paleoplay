package cmd

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/development-and-dinosaurs/paleoplay/internal"
	"github.com/spf13/cobra"
)

var postCmd = &cobra.Command{
	Use:   "post",
	Short: "Create a blog post for each screenshot specified.",
	Long: `Create a blog post for each screenshot specified.
	
This will pull screenshots from Steam, run them through the Tinify API to
compress and resize them for web hosting, and create a Hugo style blog post
for each one with the name of the game and the date of the screenshot.`,
	Run: func(cmd *cobra.Command, args []string) {
		user := cmd.Flag("user").Value.String()
		tinifyApiKey := cmd.Flag("tinify-api-key").Value.String()
		steamApiKey := cmd.Flag("steam-api-key").Value.String()
		maxImages, _ := cmd.Flags().GetInt("max-images")
		state := internal.ReadState()
		internal.InitSteamApi(steamApiKey)
		internal.InitSteam(state)
		screenshots := internal.PullPublicScreenshots(user)
		posts := internal.GroupScreenshots(screenshots)
		posts = internal.DeferRecentGroups(posts, time.Now())
		posts = internal.CapGroups(posts, maxImages)
		internal.InitTinify(tinifyApiKey)
		// Deferred so whatever did publish is still recorded if a later post
		// takes the process down, rather than the whole run being repeated.
		var published []string
		defer func() { internal.WriteState(published) }()
		failed := 0
		for _, post := range posts {
			urls, err := publishPost(post)
			if err != nil {
				fmt.Printf("Skipping post for %s on %s: %v\n", post.SteamId, post.Date, err)
				failed++
				continue
			}
			published = append(published, urls...)
		}
		fmt.Printf("Published %d posts, skipped %d\n", len(posts)-failed, failed)
	},
}

// publishPost compresses every image in the group, writes them, and then writes
// the post. A group is all or nothing: if any image fails, the post is not
// written and none of its screenshots are recorded, so the next run retries the
// whole group rather than leaving a post with images missing from it.
func publishPost(group internal.ImageGrouping) ([]string, error) {
	images := make([][]byte, len(group.Images))
	errs := make([]error, len(group.Images))
	var wg sync.WaitGroup
	for i, image := range group.Images {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tinyImage, err := internal.Tinify(image.Source)
			if err != nil {
				errs[i] = err
				return
			}
			images[i] = tinyImage
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if err := os.MkdirAll("static/images/posts/", 0777); err != nil {
		return nil, fmt.Errorf("could not create images directory: %w", err)
	}
	for i, image := range group.Images {
		destination := "static/images/posts/" + image.Destination
		if err := os.WriteFile(destination, images[i], 0777); err != nil {
			return nil, fmt.Errorf("could not write %s: %w", destination, err)
		}
	}
	if err := internal.CreatePost(group); err != nil {
		return nil, err
	}
	return internal.Map(group.Images, func(i internal.Image) string { return i.Screenshot }), nil
}

func init() {
	rootCmd.AddCommand(postCmd)
	postCmd.Flags().StringP("user", "u", "", "The Steam user to pull screenshots for (required)")
	postCmd.MarkFlagRequired("user")
	postCmd.Flags().StringP("tinify-api-key", "t", "", "The Tinify API key to use (required)")
	postCmd.MarkFlagRequired("tinify-api-key")
	postCmd.Flags().StringP("steam-api-key", "s", "", "The Steam Web API key to use (required)")
	postCmd.MarkFlagRequired("steam-api-key")
	postCmd.Flags().Int("max-images", 100,
		"Maximum images to compress in one run, rounded down to whole posts (0 for no limit)")
}
