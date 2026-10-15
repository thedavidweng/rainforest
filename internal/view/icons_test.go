package view

import (
	"slices"
	"strings"
	"testing"
)

func TestIconsMatchManifest(t *testing.T) {
	manifest, err := iconFiles.ReadFile("icons/icons.txt")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range strings.Split(string(manifest), "\n")[1:] {
		weight, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		if weight != "regular" {
			name += "-" + weight
		}
		want = append(want, name)
	}
	entries, err := iconFiles.ReadDir("icons")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".svg"); ok {
			got = append(got, name)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("icons.txt and icons/*.svg differ\nmanifest: %v\nfiles:    %v", want, got)
	}
	for _, name := range want {
		loadIcon(name)
	}
}
