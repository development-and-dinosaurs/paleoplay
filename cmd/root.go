package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "paleoplay",
	Short: "A tool to quickly and easily administer a screenshot blog.",
	Long: `Paleoplay is a tool that can be used to quickly and easily administer 
a screenshot blog.

Paleoplay can be used to initialise the initial blog site using Hugo,
pull screenshots from Steam, resize and compress images using the 
Tinify API, and create blog posts containing the images for publishing.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
