package tui

import "testing"

func TestIsSkillCheckKeyAcceptsPhysicalSpaceBar(t *testing.T) {
	for _, key := range []string{"space", " "} {
		if !isSkillCheckKey(key) {
			t.Fatalf("%q should trigger a skill check", key)
		}
	}
	if isSkillCheckKey("enter") {
		t.Fatal("enter must not trigger a skill check")
	}
}
