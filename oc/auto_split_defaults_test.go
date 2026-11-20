package oc

import "testing"

func TestFineSplitDefaults(t *testing.T) {
	names, err := autoSplitNames(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 10 {
		t.Fatalf("want 10 defaults: %v", names)
	}
	for _, name := range []string{"左眼睛", "右眼睛", "左手", "右手", "嘴", "左耳朵", "右耳朵", "头发"} {
		found := false
		for _, v := range names {
			if v == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s", name)
		}
	}
}
