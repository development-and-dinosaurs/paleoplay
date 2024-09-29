package internal

import (
	"fmt"
	"os"
	"strings"
)

var postTemplate = `---
title: %[1]s - %[2]s
date: %[1]s
image: images/posts/%[3]s
games: %[4]s
genres:
%[5]s
---
`

func Post(screenshot Screenshot) {
	genres := make([]string, 0)
	for _, g := range screenshot.Genres {
		genres = append(genres, fmt.Sprintf("  - %s", g))
	}
	post := fmt.Sprintf(postTemplate, screenshot.Date.Format("2006-01-02"), screenshot.Game, screenshot.FileName(), screenshot.Game, strings.Join(genres, "\n"))
	os.WriteFile("content/posts/"+strings.ReplaceAll(screenshot.FileName(), "webp", "md"), []byte(post), 0777)
}
