package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/development-and-dinosaurs/paleoplay/internal"
	"github.com/spf13/cobra"
)

// Version reports the module version stamped in by `go install module@version`.
// It is the only reliable way to tell which build is actually running when
// Paleoplay is invoked from a pre-built Docker image, where the binary can be
// far older than the Dockerfile that supposedly pinned it. Builds from source
// report "(devel)".
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "unknown"
	}
	return info.Main.Version
}

var rootCmd = &cobra.Command{
	Use:     "paleoplay",
	Version: Version(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Welcome to Paleoplay %s (post format v%d)\n", Version(), internal.PaleoplayVersion)
	},
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
