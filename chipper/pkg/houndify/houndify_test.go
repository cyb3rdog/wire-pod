package houndify

import "testing"

// TestParseSpokenResponseNoPanic guards against a regression of the panics
// that chained single-value type assertions used to cause on unexpected or
// error-shaped Houndify responses.
func TestParseSpokenResponseNoPanic(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		want    string
		wantErr bool
	}{
		{
			name: "well-formed",
			json: `{"Status":"OK","NumToReturn":1,"AllResults":[{"SpokenResponseLong":"hello"}]}`,
			want: "hello",
		},
		{"not json", `not json`, "", true},
		{"missing Status", `{}`, "", true},
		{"error status", `{"Status":"Error","ErrorMessage":"bad request"}`, "", true},
		{"zero results", `{"Status":"OK","NumToReturn":0}`, "", true},
		{"missing AllResults", `{"Status":"OK","NumToReturn":1}`, "", true},
		{"empty AllResults", `{"Status":"OK","NumToReturn":1,"AllResults":[]}`, "", true},
		{"malformed result entry", `{"Status":"OK","NumToReturn":1,"AllResults":["not an object"]}`, "", true},
		{"missing SpokenResponseLong", `{"Status":"OK","NumToReturn":1,"AllResults":[{}]}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseSpokenResponse(c.json)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got result=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
