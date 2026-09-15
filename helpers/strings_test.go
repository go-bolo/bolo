package helpers

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// STR-01
func TestTruncateString(t *testing.T) {
	t.Run("truncates ascii text respecting the omission suffix", func(t *testing.T) {
		got := TruncateString("hello world", 5, "...")

		assert.Equal(t, "hello...", got)
	})

	t.Run("returns the original text when it fits", func(t *testing.T) {
		assert.Equal(t, "abc", TruncateString("abc", 10, "..."))
		assert.Equal(t, "abc", TruncateString("abc", 3, "..."))
	})

	t.Run("returns empty for empty text", func(t *testing.T) {
		assert.Equal(t, "", TruncateString("", 10, "..."))
	})

	t.Run("returns empty for zero or negative length", func(t *testing.T) {
		assert.Equal(t, "", TruncateString("hello", 0, "..."))
		assert.Equal(t, "", TruncateString("hello", -1, "..."))
	})

	t.Run("truncates unicode text by runes keeping the suffix convention", func(t *testing.T) {
		got := TruncateString("héllo wörld", 5, "…")

		assert.True(t, utf8.ValidString(got),
			"truncated output must be valid utf-8, got %q", got)
		assert.Equal(t, "héllo…", got,
			"truncation should count runes, not bytes")
	})

	t.Run("never emits invalid utf-8 when cutting accented runes", func(t *testing.T) {
		got := TruncateString("héllo", 2, "...")

		assert.True(t, utf8.ValidString(got),
			"truncated output must be valid utf-8, got %q", got)
		assert.Equal(t, "hé...", got)
	})
}
