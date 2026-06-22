package scraper

import (
	"testing"
)

// listSampleHTML mirrors the real structure of a flingtrainer.com list page
// (article.post-standard with h2.post-title a, and the real cover image class).
const listSampleHTML = `
<html><body>
<article class="group post-standard post-39208 post type-post status-publish format-standard has-post-thumbnail hentry category-trainer tag-bellwright">
  <h2 class="post-title"><a href="https://flingtrainer.com/trainer/bellwright-trainer/">Bellwright Trainer</a></h2>
  <div class="post-details">
    <div class="post-details-thumb"><img src="" /></div>
    <img class="attachment-stylizer-small size-stylizer-small wp-post-image" src="https://flingtrainer.com/wp-content/uploads/2024/04/header-11-200x200.jpg" />
    <div class="entry">27 Options · Game Version: Early Access+ · Last Updated: 2026.06.15</div>
    <div class="post-details-date">
      <span class="post-details-day">15</span>
      <span class="post-details-month">Jun</span>
      <span class="post-details-year">2026</span>
    </div>
  </div>
</article>
</body></html>
`

// detailSampleHTML mirrors the real detail page: a summary line in .entry > p
// and a .download-attachments table with multiple version rows.
const detailSampleHTML = `
<html><body>
<h1 class="post-title">Bellwright Trainer</h1>
<div class="entry">
  <p>27 Options · Game Version: Early Access+ · Last Updated: 2026.06.15<span id="more-39208"></span></p>
  <table class="download-attachments style-table">
    <thead><tr>
      <th>File</th><th>Date added</th><th>File size</th><th>Downloads</th>
    </tr></thead>
    <tbody>
      <tr>
        <td><img class="attachment-icon" src="https://flingtrainer.com/wp-content/uploads/icon.png" /></td>
        <td><a href="https://flingtrainer.com/downloads/G2HwTO7ASy968AdMdyJhsw,,">Bellwright.Early.Access.Plus.27.Trainer.Updated.2026.06.15-FLiNG</a></td>
        <td>2026-06-15 07:51</td>
        <td>855 KB</td>
        <td>1115</td>
      </tr>
      <tr>
        <td><img class="attachment-icon" src="https://flingtrainer.com/wp-content/uploads/icon.png" /></td>
        <td><a href="https://flingtrainer.com/downloads/zpsjizoWxu9VkiESyJW8bg,,">Bellwright.Early.Access.Plus.27.Trainer.Updated.2025.04.29-FLiNG</a></td>
        <td>2025-04-29 20:20</td>
        <td>843 KB</td>
        <td>37896</td>
      </tr>
    </tbody>
  </table>
</div>
</body></html>
`

func TestParseTrainerList(t *testing.T) {
	games, err := ParseTrainerList(listSampleHTML)
	if err != nil {
		t.Fatalf("ParseTrainerList error: %v", err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	g := games[0]
	if g.NameEN != "Bellwright Trainer" {
		t.Errorf("NameEN = %q, want %q", g.NameEN, "Bellwright Trainer")
	}
	if g.SourceID != "bellwright-trainer" {
		t.Errorf("SourceID = %q, want %q", g.SourceID, "bellwright-trainer")
	}
	if g.SourceURL != "https://flingtrainer.com/trainer/bellwright-trainer/" {
		t.Errorf("SourceURL = %q", g.SourceURL)
	}
	// The .post-details-thumb img has empty src; we should fall back to wp-post-image.
	if g.CoverURL != "https://flingtrainer.com/wp-content/uploads/2024/04/header-11-200x200.jpg" {
		t.Errorf("CoverURL = %q (expected wp-post-image fallback)", g.CoverURL)
	}
	if g.OptionsNum != 27 {
		t.Errorf("OptionsNum = %d, want 27", g.OptionsNum)
	}
	if g.UpdatedAt == 0 {
		t.Errorf("UpdatedAt should be non-zero for a valid date")
	}
}

func TestParseTrainerDetail(t *testing.T) {
	page, err := ParseTrainerDetail(detailSampleHTML)
	if err != nil {
		t.Fatalf("ParseTrainerDetail error: %v", err)
	}
	if page.Options != "27 Options" {
		t.Errorf("Options = %q, want %q", page.Options, "27 Options")
	}
	if page.GameVersion != "Early Access+" {
		t.Errorf("GameVersion = %q, want %q", page.GameVersion, "Early Access+")
	}
	if page.UpdatedAt == 0 {
		t.Errorf("UpdatedAt should be parsed from Last Updated")
	}

	if len(page.Trainers) != 2 {
		t.Fatalf("expected 2 trainer versions, got %d", len(page.Trainers))
	}

	latest := page.Trainers[0]
	if latest.DownloadURL != "https://flingtrainer.com/downloads/G2HwTO7ASy968AdMdyJhsw,," {
		t.Errorf("DownloadURL = %q", latest.DownloadURL)
	}
	if latest.FileName != "Bellwright.Early.Access.Plus.27.Trainer.Updated.2026.06.15-FLiNG" {
		t.Errorf("FileName = %q", latest.FileName)
	}
	if latest.FileSize != 855*1024 {
		t.Errorf("FileSize = %d, want %d", latest.FileSize, 855*1024)
	}
	if latest.DownloadCount != 1115 {
		t.Errorf("DownloadCount = %d, want 1115", latest.DownloadCount)
	}
	if latest.GameVersion != "Early Access+" {
		t.Errorf("GameVersion on trainer = %q", latest.GameVersion)
	}
	if latest.SourceHash == "" {
		t.Errorf("SourceHash should be set from download URL")
	}

	older := page.Trainers[1]
	if older.DownloadCount != 37896 {
		t.Errorf("older DownloadCount = %d, want 37896", older.DownloadCount)
	}
	// versions should have distinct dedup hashes (different URLs)
	if older.SourceHash == latest.SourceHash {
		t.Errorf("versions should have distinct SourceHash values")
	}
}

// notFoundHTML mirrors the real 404 page: it has the site stylesheet
// (which references ".post-standard") but NO real game <article> elements.
// A naive substring check would wrongly report articles here.
const notFoundHTML = `<html><head>
<title>Page not found - FLiNG Trainer</title>
<style>
.post-standard:hover .post-title a,
.blog .post-standard, .single .post-standard, .archive .post-standard, .search .post-standard {
  background:#fff;
}
</style></head><body>
<div class="content">Sorry, the page you requested does not exist.</div>
</body></html>`

func TestHasGameArticles(t *testing.T) {
	// Real list page HTML must report true.
	if !hasGameArticles(listSampleHTML) {
		t.Errorf("hasGameArticles(listSampleHTML) = false, want true")
	}
	// 404 / not-found page must report false even though it ships
	// ".post-standard" CSS rules.
	if hasGameArticles(notFoundHTML) {
		t.Errorf("hasGameArticles(notFoundHTML) = true, want false (CSS-only references must not count)")
	}
	// Trivially empty page.
	if hasGameArticles("") {
		t.Errorf("hasGameArticles(\"\") = true, want false")
	}
}

func TestParseFileSize(t *testing.T) {
	cases := []struct {
		in   string
		want int32
	}{
		{"855 KB", 855 * 1024},
		{"1 MB", 1024 * 1024},
		// FileSize is int32; keep GB case within range
		{"1 GB", 1024 * 1024 * 1024},
		{"512 bytes", 512},
	}
	for _, c := range cases {
		got := parseFileSize(c.in)
		if got != c.want {
			t.Errorf("parseFileSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
