package settings

import "testing"

func TestDirectoryFileLimit(t *testing.T) {
	cases := []struct {
		name string
		set  int
		want int
	}{
		{"positive is a cap", 750, 750},
		{"small positive is a valid cap", 5, 5},
		{"one is a valid cap", 1, 1},
		{"zero is unlimited", 0, 0},
		{"default is preserved", 500, 500},
		{"negative falls back to default", -1, defaultSettings.MaxDirectoryFiles},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Settings{MaxDirectoryFiles: tc.set}
			if got := s.DirectoryFileLimit(); got != tc.want {
				t.Errorf("DirectoryFileLimit(%d) = %d, want %d", tc.set, got, tc.want)
			}
		})
	}
}
