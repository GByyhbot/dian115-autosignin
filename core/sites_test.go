package core

import "testing"

func TestMatchPreset(t *testing.T) {
	cases := []struct {
		url, key string
	}{
		{"https://52pt.site", "52pt"},
		{"https://www.52pt.site/index.php", "52pt"},
		{"https://pt.btschool.club", "btschool"},
		{"https://ptchdbits.co", "chdbits"},
		{"https://haidan.cc", "haidan"},
		{"https://club.hares.top", "hares"},
		{"https://hdarea.club", "hdarea"},
		{"https://hdchina.org", "hdchina"},
		{"https://hdcity.city", "hdcity"},
		{"https://hdsky.me", "hdsky"},
		{"https://pt.hdupt.com", "hdupt"},
		{"https://m-team.cc", "mteam"},
		{"https://pt.m-team.cc", "mteam"},
		{"https://v6.nexushd.org", "nexushd"},
		{"https://open.cd", "opencd"},
		{"https://pterclub.com", "pterclub"},
		{"https://www.pttime.org", "pttime"},
		{"https://ptzone.xyz", "ptzone"},
		{"https://rousi.pro", "rousi"},
		{"https://tjupt.org", "tjupt"},
		{"https://totheglory.im", "ttg"},
		{"https://u2.dmhy.org", "u2"},
		{"https://yemapt.org", "yema"},
		{"https://zhuque.in", "zhuque"},
		{"https://example.com", ""},
	}
	for _, c := range cases {
		if got := MatchPreset(c.url); got != c.key {
			t.Errorf("MatchPreset(%s) = %q, want %q", c.url, got, c.key)
		}
	}
}

func TestPresetByKey(t *testing.T) {
	if len(BuiltinPresets) != 22 {
		t.Fatalf("builtin presets = %d, want 22", len(BuiltinPresets))
	}
	if _, ok := PresetByKey("hdchina"); !ok {
		t.Fatal("hdchina preset missing")
	}
	if _, ok := PresetByKey("nope"); ok {
		t.Fatal("unknown preset must not resolve")
	}
}

func TestResolveSiteIDs(t *testing.T) {
	c := Config{Sites: []Site{
		{ID: "a"}, {ID: "b"}, {ID: "c"},
	}}
	got := c.ResolveSiteIDs([]string{"b", "all", "nope"})
	if len(got) != 3 || got[0] != "b" || got[1] != "a" || got[2] != "c" {
		t.Fatalf("resolve = %v", got)
	}
}
