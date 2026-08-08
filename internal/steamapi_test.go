package internal

import (
	"encoding/json"
	"testing"
	"time"
)

// The exact payload shape returned by GetUserFiles, from a real response.
const sample = `{"response":{"total":1904,"publishedfiledetails":[
 {"publishedfileid":"3779405125","consumer_appid":345350,"app_name":"LIGHTNING RETURNS: FINAL FANTASY XIII",
  "file_url":"https://images.steamusercontent.com/ugc/13500203713475830115/D37BD14475B864AEF8C23AF0CF7C3AB0DB40D7F2/",
  "time_created":1786135075}]}}`

func TestParsesUserFilesIntoScreenshot(t *testing.T) {
	var r userFilesResponse
	if err := json.Unmarshal([]byte(sample), &r); err != nil {
		t.Fatal(err)
	}
	if r.Response.Total != 1904 || len(r.Response.Files) != 1 {
		t.Fatalf("total=%d files=%d", r.Response.Total, len(r.Response.Files))
	}
	s := r.Response.Files[0].screenshot()

	// This must match the format already in .state or everything reprocesses.
	want := "https://steamcommunity.com/sharedfiles/filedetails/?id=3779405125"
	if s.URL != want {
		t.Fatalf("URL = %q, want %q", s.URL, want)
	}
	if s.SteamId != "345350" {
		t.Fatalf("SteamId = %q", s.SteamId)
	}
	if s.Game != "LIGHTNING RETURNS: FINAL FANTASY XIII" {
		t.Fatalf("Game = %q", s.Game)
	}
	if got := s.Date.Format("2006-01-02"); got != "2026-08-07" {
		t.Fatalf("Date = %s, want 2026-08-07", got)
	}
	t.Logf("parsed: %s | %s | %s", s.Date.Format("2006-01-02"), s.Game, s.URL)
}

func TestStateFormatMatchesExistingEntries(t *testing.T) {
	// A real line from the user's .state file.
	existing := "https://steamcommunity.com/sharedfiles/filedetails/?id=2409280695"
	if got := screenshotUrl("2409280695"); got != existing {
		t.Fatalf("rebuilt %q, state has %q", got, existing)
	}
}

func group(date string, steamId string, n int) ImageGrouping {
	imgs := make([]Image, n)
	for i := range imgs {
		imgs[i] = Image{Source: "s", Destination: "d", Screenshot: "u"}
	}
	return ImageGrouping{SteamId: steamId, Date: date, Images: imgs}
}

func TestCapGroupsNeverSplitsAPost(t *testing.T) {
	groups := []ImageGrouping{group("2026-01-01", "1", 3), group("2026-01-02", "2", 3), group("2026-01-03", "3", 3)}

	got := CapGroups(groups, 7)
	if len(got) != 2 {
		t.Fatalf("cap 7 kept %d posts, want 2 (3+3=6 fits, +3=9 does not)", len(got))
	}
	total := 0
	for _, g := range got {
		total += len(g.Images)
	}
	if total != 6 {
		t.Fatalf("kept %d images, want 6", total)
	}
}

func TestCapGroupsAlwaysAllowsFirstPost(t *testing.T) {
	groups := []ImageGrouping{group("2026-01-01", "1", 50), group("2026-01-02", "2", 1)}
	got := CapGroups(groups, 10)
	if len(got) != 1 || len(got[0].Images) != 50 {
		t.Fatalf("oversized first post was dropped, would stall forever: %d posts", len(got))
	}
}

func TestCapGroupsZeroMeansUnlimited(t *testing.T) {
	groups := []ImageGrouping{group("2026-01-01", "1", 500)}
	if got := CapGroups(groups, 0); len(got) != 1 {
		t.Fatalf("max=0 should not cap")
	}
}

func TestGroupScreenshotsIsDeterministicallyOrdered(t *testing.T) {
	mk := func(day int, game string) Screenshot {
		return Screenshot{
			URL: game + string(rune('0'+day)), Game: game, SteamId: "1",
			Date: time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC),
		}
	}
	shots := []Screenshot{mk(3, "c"), mk(1, "a"), mk(2, "b")}
	for i := 0; i < 20; i++ {
		g := GroupScreenshots(shots)
		if len(g) != 3 || g[0].Date != "2026-01-01" || g[2].Date != "2026-01-03" {
			t.Fatalf("iteration %d not sorted oldest-first: %v", i, []string{g[0].Date, g[1].Date, g[2].Date})
		}
	}
}

func TestDeferRecentGroupsHoldsBackToday(t *testing.T) {
	now := time.Date(2026, 8, 8, 6, 0, 0, 0, time.UTC)
	groups := []ImageGrouping{
		group("2026-08-06", "1", 2),
		group("2026-08-07", "2", 2),
		group("2026-08-08", "3", 2), // today, still accumulating
	}
	got := DeferRecentGroups(groups, now)
	if len(got) != 2 {
		t.Fatalf("kept %d posts, want 2", len(got))
	}
	for _, g := range got {
		if g.Date >= "2026-08-08" {
			t.Fatalf("published a post dated %s on %s", g.Date, "2026-08-08")
		}
	}
}

func TestDeferRecentGroupsUsesUtcNotLocal(t *testing.T) {
	// 00:30 BST on the 8th is 23:30 UTC on the 7th, so the 7th is still open.
	bst := time.FixedZone("BST", 3600)
	now := time.Date(2026, 8, 8, 0, 30, 0, 0, bst)
	groups := []ImageGrouping{group("2026-08-07", "1", 1)}
	if got := DeferRecentGroups(groups, now); len(got) != 0 {
		t.Fatalf("published the 7th while it was still 23:30 UTC on the 7th")
	}
}

func TestDeferRecentGroupsAlsoHoldsFutureDates(t *testing.T) {
	now := time.Date(2026, 8, 8, 6, 0, 0, 0, time.UTC)
	groups := []ImageGrouping{group("2026-08-09", "1", 1)}
	if got := DeferRecentGroups(groups, now); len(got) != 0 {
		t.Fatalf("published a future-dated post; clock skew would slip through")
	}
}

func TestDeferThenCapKeepsPostsWhole(t *testing.T) {
	now := time.Date(2026, 8, 8, 6, 0, 0, 0, time.UTC)
	groups := []ImageGrouping{
		group("2026-08-05", "1", 3),
		group("2026-08-06", "2", 3),
		group("2026-08-07", "3", 3),
		group("2026-08-08", "4", 3),
	}
	got := CapGroups(DeferRecentGroups(groups, now), 7)
	if len(got) != 2 {
		t.Fatalf("kept %d posts, want 2", len(got))
	}
	images := 0
	for _, g := range got {
		images += len(g.Images)
		if len(g.Images) != 3 {
			t.Fatalf("post was split: %d images", len(g.Images))
		}
	}
	if images != 6 {
		t.Fatalf("%d images, want 6", images)
	}
}
