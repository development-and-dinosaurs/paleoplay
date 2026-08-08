package internal

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

func renderFrontMatter(t *testing.T, p PostData) map[string]any {
	t.Helper()
	tmpl, err := template.New("post").Parse(postTemplate)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, p); err != nil {
		t.Fatal(err)
	}
	var into map[string]any
	if err := yaml.Unmarshal([]byte(strings.Trim(b.String(), "-\n")), &into); err != nil {
		t.Fatalf("front matter is not valid YAML: %v\n\n%s", err, b.String())
	}
	return into
}

// Steam names routinely contain colons, which YAML reads as a second mapping
// value. This broke the Hugo build on "Teenage Mutant Ninja Turtles: Splintered
// Fate", whose franchise carries the colon too.
func TestFrontMatterSurvivesColonsEverywhere(t *testing.T) {
	fm := renderFrontMatter(t, PostData{
		Date:       "2026-07-14",
		Images:     []string{"images/posts/a-1.webp"},
		Game:       "Teenage Mutant Ninja Turtles: Splintered Fate",
		Franchise:  "Teenage Mutant Ninja Turtles: Splintered Fate",
		Genres:     []string{"Action: Extreme"},
		Developers: []string{"Dev: One"},
		Publishers: []string{"Pub: Two"},
		Tags:       []string{"Co-op", "Beat 'em up", "Tag: With Colon", `Quote"Tag`},
		SteamId:    "2996040",
		Paleoplay:  2,
	})
	if got := fm["franchises"]; got != "Teenage Mutant Ninja Turtles: Splintered Fate" {
		t.Fatalf("franchises = %v", got)
	}
	if got := fm["games"]; got != "Teenage Mutant Ninja Turtles: Splintered Fate" {
		t.Fatalf("games = %v", got)
	}
	tags := fm["tags"].([]any)
	if len(tags) != 4 || tags[2] != "Tag: With Colon" || tags[3] != `Quote"Tag` {
		t.Fatalf("tags round-tripped wrong: %v", tags)
	}
}

// An absent franchise must stay null rather than becoming an empty taxonomy term.
func TestFrontMatterEmptyFranchiseStaysNull(t *testing.T) {
	fm := renderFrontMatter(t, PostData{
		Date: "2026-07-14", Game: "Fields of Mistria", Franchise: "",
		Images: []string{"images/posts/a-1.webp"}, SteamId: "2142790", Paleoplay: 2,
	})
	if v, ok := fm["franchises"]; !ok || v != nil {
		t.Fatalf("empty franchise should be null, got %#v", v)
	}
}

func TestFrontMatterApostropheTagUnchanged(t *testing.T) {
	fm := renderFrontMatter(t, PostData{
		Date: "2026-07-14", Game: "G", Images: []string{"i"},
		Tags: []string{"Beat 'em up"}, SteamId: "1", Paleoplay: 2,
	})
	if fm["tags"].([]any)[0] != "Beat 'em up" {
		t.Fatalf("apostrophe tag mangled: %v", fm["tags"])
	}
}
