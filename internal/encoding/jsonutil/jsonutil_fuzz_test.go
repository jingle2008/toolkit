package jsonutil

import (
	"encoding/json"
	"testing"
)

func FuzzPrettyJSON(f *testing.F) {
	// Seed with some basic JSON-able values
	f.Add(`{"foo": "bar"}`)
	f.Add(`123`)
	f.Add(`true`)
	f.Add(`null`)
	f.Add(`[1,2,3]`)
	f.Fuzz(func(_ *testing.T, input string) {
		var v any
		_ = json.Unmarshal([]byte(input), &v)
		_, _ = PrettyJSON(v)
	})
}
