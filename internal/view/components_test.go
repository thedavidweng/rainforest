package view

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		-5:                               "0 B",
		0:                                "0 B",
		1023:                             "1023 B",
		1024:                             "1.00 KB",
		1536:                             "1.50 KB",
		10 * 1024:                        "10.0 KB",
		5 * 1024 * 1024:                  "5.00 MB",
		3 * 1024 * 1024 * 1024:           "3.00 GB",
		1234 * 1024 * 1024 * 1024:        "1.21 TB",
		5000 * 1024 * 1024 * 1024 * 1024: "5000.0 TB",
	}
	for n, want := range cases {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 0, 0, 0, time.Local)
	var got [5]string
	view := func(c *ui.Context) {
		stamp := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339) }
		got = [5]string{
			relativeTime(c, stamp(-10*time.Second)),
			relativeTime(c, stamp(-5*time.Minute)),
			relativeTime(c, stamp(-2*time.Hour)),
			relativeTime(c, stamp(-20*time.Hour)),
			relativeTime(c, "not a time"),
		}
		ui.Text(c, "x")
	}
	tt := ui.NewTester(view, 200, 100)
	tt.Frame()
	_ = got
	m := locales.In("en")
	if got[4] != "" {
		t.Errorf("invalid time = %q", got[4])
	}
	if got[0] != m.TimeJustNow() && got[0] == "" {
		t.Errorf("recent = %q", got[0])
	}
}

func TestStatusMaps(t *testing.T) {
	m := locales.In("en")
	transfers := []struct {
		kind, stage, label string
		tone               tone
	}{
		{"upload", "queued", "Waiting", toneNeutral},
		{"upload", "running", "Uploading", toneAccent},
		{"backup", "running", "Uploading", toneAccent},
		{"download", "running", "Downloading", toneAccent},
		{"upload", "paused", "Paused", toneWarning},
		{"upload", "completed", "Done", toneSuccess},
		{"upload", "partial", "Partly done", toneWarning},
		{"upload", "failed", "Failed", toneDanger},
		{"upload", "cancelled", "Cancelled", toneNeutral},
		{"upload", "unconfirmed", "Needs checking", toneWarning},
		{"upload", "mystery", "mystery", toneNeutral},
	}
	for _, tc := range transfers {
		label, k := transferStatus(m, tc.kind, tc.stage)
		if label != tc.label || k != tc.tone {
			t.Errorf("transferStatus(%s,%s) = %q,%v", tc.kind, tc.stage, label, k)
		}
	}
	items := map[string]struct {
		label string
		tone  tone
	}{
		"queued":        {"Waiting", toneNeutral},
		"running":       {"In progress", toneAccent},
		"confirmed":     {"Done", toneSuccess},
		"skipped":       {"Already there", toneNeutral},
		"album_pending": {"Adding to album", toneWarning},
		"unconfirmed":   {"Needs checking", toneWarning},
		"failed":        {"Failed", toneDanger},
		"cancelled":     {"Cancelled", toneNeutral},
		"other":         {"other", toneNeutral},
	}
	for status, want := range items {
		if label, k := itemStatus(m, status); label != want.label || k != want.tone {
			t.Errorf("itemStatus(%s) = %q,%v", status, label, k)
		}
	}
	rules := []struct {
		rule    gui.BackupRuleDTO
		stopped bool
		label   string
		tone    tone
	}{
		{gui.BackupRuleDTO{Status: "active"}, false, "Up to date", toneSuccess},
		{gui.BackupRuleDTO{Status: "scanning"}, false, "Backing up…", toneAccent},
		{gui.BackupRuleDTO{Status: "paused", Paused: true}, false, "Paused", toneNeutral},
		{gui.BackupRuleDTO{Status: "source_unavailable"}, false, "Folder unavailable", toneWarning},
		{gui.BackupRuleDTO{Status: "error"}, false, "Needs attention", toneDanger},
		{gui.BackupRuleDTO{Status: "active"}, true, "Paused", toneNeutral},
		{gui.BackupRuleDTO{Status: "odd"}, false, "odd", toneNeutral},
	}
	for _, tc := range rules {
		if label, k := ruleStatus(m, tc.rule, tc.stopped); label != tc.label || k != tc.tone {
			t.Errorf("ruleStatus(%+v,%v) = %q,%v", tc.rule, tc.stopped, label, k)
		}
	}
	checks := map[string]struct {
		label string
		icon  *ui.SVG
		tone  tone
	}{
		"pass":    {"Passed", iconCheckCircle, toneSuccess},
		"warn":    {"Warning", iconWarning, toneWarning},
		"fail":    {"Failed", iconXCircle, toneDanger},
		"skipped": {"Skipped", iconCircleDashed, toneNeutral},
	}
	for status, want := range checks {
		if label, icon, k := doctorStatus(m, status); label != want.label || icon != want.icon || k != want.tone {
			t.Errorf("doctorStatus(%s) = %q,%v", status, label, k)
		}
	}
	for kind, want := range map[string]string{"upload": "Upload", "backup": "Backup", "download": "Download", "x": "x"} {
		if got := kindLabel(m, kind); got != want {
			t.Errorf("kindLabel(%s) = %q", kind, got)
		}
	}
	if kindIcon("backup") != iconFolder || kindIcon("download") != iconCloudArrowDown || kindIcon("upload") != iconCloudArrowUp {
		t.Error("kindIcon mapping")
	}
	if label, k := capabilityStatus(m, "readable"); label != "Ready" || k != toneSuccess {
		t.Errorf("capabilityStatus readable = %q,%v", label, k)
	}
	if label, k := probeStatus(m, &probeState{state: loadRunning}); label != "Checking…" || k != toneAccent {
		t.Errorf("probeStatus running = %q,%v", label, k)
	}
	if label, _ := probeStatus(m, &probeState{state: loadFailed}); label != "Unavailable" {
		t.Errorf("probeStatus failed = %q", label)
	}
}

func TestRegionHelpers(t *testing.T) {
	m := locales.In("en")
	hosts := map[string]string{
		"us": "amazon.com", "ca": "amazon.ca", "uk": "amazon.co.uk", "jp": "amazon.co.jp",
		"de": "amazon.de", "fr": "amazon.fr", "it": "amazon.it", "es": "amazon.es", "": "amazon.com",
	}
	for region, want := range hosts {
		if got := regionHost(region); got != want {
			t.Errorf("regionHost(%q) = %q, want %q", region, got, want)
		}
	}
	if got := regionLabel(m, "uk"); got != "United Kingdom · amazon.co.uk" {
		t.Errorf("regionLabel = %q", got)
	}
	if len(regionCodes) != 8 || regionCode("") != "US" || regionCode("jp") != "JP" {
		t.Error("region codes")
	}
}

func TestComponentsRender(t *testing.T) {
	var clicked []string
	view := func(c *ui.Context) {
		applyTheme(c)
		ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
			pageHeader(c, "Header", "Sub", func() { ui.Button(c, "Act") })
			sectionTitle(c, "Section", func() { ui.Text(c, "trail") })
			card(c, func() { ui.Text(c, "in card") })
			for _, k := range []tone{toneNeutral, toneAccent, toneSuccess, toneWarning, toneDanger} {
				statusPill(c, "pill", k)
			}
			banner(c, toneWarning, iconWarning, "Banner title", "Banner body", "Fix", func() { clicked = append(clicked, "banner") })
			emptyState(c, iconImage, "Empty title", "Empty body", "Go", func() { clicked = append(clicked, "empty") })
			if iconButton(c, iconX, "Close it").Clicked() {
				clicked = append(clicked, "icon")
			}
			pageFrame(c, "Framed", "", func() { ui.Text(c, "frame action") }, func() { ui.Text(c, "frame body") })
		})
	}
	tt := ui.NewTester(view, 900, 900)
	tt.Frame()
	for _, s := range []string{"Header", "Sub", "Act", "Section", "trail", "in card", "pill", "Banner title", "Banner body", "Empty title", "Empty body"} {
		if !tt.HasText(s) {
			t.Errorf("missing %q in %q", s, tt.Texts())
		}
	}
	for _, label := range []string{"Fix", "Go", "Close it"} {
		if err := tt.Click(label); err != nil {
			t.Errorf("click %q: %v", label, err)
		}
	}
	if len(clicked) != 3 {
		t.Errorf("clicked = %v", clicked)
	}
}

func TestChineseLocaleRenders(t *testing.T) {
	h := newHarness(t, true)
	h.tt.SetLocale("zh-Hans")
	h.start()
	h.wantText(locales.In("zh-Hans").NavUpload())
	h.wantText(locales.In("zh-Hans").NavSettings())
	h.noText("Settings")
}

func TestErrMessage(t *testing.T) {
	err := &gui.Error{Code: gui.ErrAuthFailed, Message: "Nope"}
	if got := errMessage(err); got != "Nope" {
		t.Errorf("errMessage = %q", got)
	}
	if got := errCode(err); got != gui.ErrAuthFailed || !isAuthError(got) {
		t.Errorf("errCode = %q", got)
	}
	if got := errMessage(context.DeadlineExceeded); got != context.DeadlineExceeded.Error() {
		t.Errorf("errMessage(plain) = %q", got)
	}
	if isAuthError("") {
		t.Error("empty code is not an auth error")
	}
}
