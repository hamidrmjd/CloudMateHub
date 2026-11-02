package envs

import (
	"strings"
	"testing"
)

func TestAnnouncementFor(t *testing.T) {
	cases := []struct {
		edition   Edition
		wantTitle string
	}{
		{Community, "Welcome to CloudMateHub Community!"},
		{Enterprise, "Welcome to CloudMateHub Enterprise!"},
		{Cloud, "Welcome to CloudMateHub Cloud!"},
		{"", "Welcome to CloudMateHub Community!"},
	}

	for _, tc := range cases {
		t.Run(string(tc.edition), func(t *testing.T) {
			got := AnnouncementFor(tc.edition)

			if !strings.Contains(got, tc.wantTitle) {
				t.Errorf("expected title %q in announcement, got:\n%s", tc.wantTitle, got)
			}

			if strings.Contains(strings.ToLower(got), "shellhub") {
				t.Errorf("welcome banner still contains upstream branding: %s", got)
			}

		})
	}
}
