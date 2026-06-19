package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase62(t *testing.T) {
	t.Parallel()
	cases := map[int64]string{
		0:             "0",
		1:             "1",
		61:            "z",
		62:            "10",
		-1:            "-1",
		1781787680652: "VMtrWcG", // from the OCI MQL reference (startMs)
		1781832733444: "VMwuYuC", // from the OCI MQL reference (endMs)
	}
	for n, want := range cases {
		assert.Equal(t, want, base62(n), "base62(%d)", n)
	}
}
