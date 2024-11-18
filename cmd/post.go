package cmd

import (
	"os"
	"sync"

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
		state := internal.ReadState()
		internal.InitSteam(state)
		screenshots := internal.PullPublicScreenshots(user)
		posts := internal.GroupScreenshots(screenshots)
		internal.InitTinify(tinifyApiKey)
		var wg sync.WaitGroup
		for _, post := range posts {
			for _, image := range post.Images {
				wg.Add(1)
				go func() {
					defer wg.Done()
					tinyImage := internal.Tinify(image.Source)
					os.MkdirAll("static/images/posts/", 0777)
					os.WriteFile("static/images/posts/"+image.Destination, tinyImage, 0777)
				}()
			}
			internal.CreatePost(post)
		}
		wg.Wait()
		newState := internal.Map(screenshots, func(s internal.Screenshot) string { return s.URL })
		internal.WriteState(newState)
		internal.CloseSteam()
	},
}

func init() {
	rootCmd.AddCommand(postCmd)
	postCmd.Flags().StringP("user", "u", "", "The Steam user to pull screenshots for (required)")
	postCmd.MarkFlagRequired("user")
	postCmd.Flags().StringP("tinify-api-key", "t", "", "The Tinify API key to use (required)")
	postCmd.MarkFlagRequired("tinify-api-key")
}
