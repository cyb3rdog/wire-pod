package jdocsserver

import "testing"

// TestEsnFromThingNoPanic guards against a regression of the panic that a
// robot-supplied Thing string with no colon used to cause via a bare
// strings.Split(...)[1] index.
func TestEsnFromThingNoPanic(t *testing.T) {
	cases := []struct {
		thing   string
		want    string
		wantErr bool
	}{
		{"vic:00e20145", "00e20145", false},
		{"vic:", "", true},
		{"malformed", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := esnFromThing(c.thing)
		if c.wantErr {
			if err == nil {
				t.Errorf("esnFromThing(%q): expected error, got esn=%q", c.thing, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("esnFromThing(%q): unexpected error: %v", c.thing, err)
		}
		if got != c.want {
			t.Errorf("esnFromThing(%q) = %q, want %q", c.thing, got, c.want)
		}
	}
}
