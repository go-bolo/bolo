package helpers

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractYearFromText(t *testing.T) {
	t.Run("Should extract year from fileName", func(t *testing.T) {
		mockText := "dfp_cia_aberta_2011.zip"
		result := ExtractYearFromText(mockText)
		assert.EqualValues(t, result, "2011")
	})

	t.Run("Should return a empty string without any year on text", func(t *testing.T) {
		mockText := "dfp_cia_aberta.zip"
		result := ExtractYearFromText(mockText)
		assert.EqualValues(t, result, "")
	})

	t.Run("Should return a empty string without any valid year on text", func(t *testing.T) {
		mockText := "dfp_cia_aberta_22_22.zip"
		result := ExtractYearFromText(mockText)
		assert.EqualValues(t, result, "")
	})
}

func TestFormatCurrencyDate(t *testing.T) {
	// t.Run("Should return a currency date", func(t *testing.T) {
	// 	layout := "02-01-2006"
	// 	FormatCurrencyDate(layout)
	// })

	t.Run("Should return error with invalid format", func(t *testing.T) {
		layout := ""
		result := FormatCurrencyDate(layout)
		assert.Equal(t, "", result)
	})

}

// DATE-01
func TestFormatDate(t *testing.T) {
	const layout = "2006-01-02 15:04"

	newUTCDate := func() *time.Time {
		date := time.Date(2023, 7, 16, 12, 30, 0, 0, time.UTC)
		return &date
	}

	t.Run("formats the date on the configured valid timezone", func(t *testing.T) {
		t.Setenv("SITE_TIMEZONE", "America/Sao_Paulo")

		date := newUTCDate()

		loc, err := time.LoadLocation("America/Sao_Paulo")
		require.NoError(t, err)
		expected := date.In(loc).Format(layout)

		got := FormatDate(date, layout)

		assert.Equal(t, "2023-07-16 09:30", got)
		assert.Equal(t, expected, got)
	})

	t.Run("formats on UTC when SITE_TIMEZONE is absent", func(t *testing.T) {
		// empty string may not be equivalent to absent, so unset explicitly with restore
		previous, hadPrevious := os.LookupEnv("SITE_TIMEZONE")
		require.NoError(t, os.Unsetenv("SITE_TIMEZONE"))
		t.Cleanup(func() {
			if hadPrevious {
				_ = os.Setenv("SITE_TIMEZONE", previous)
			} else {
				_ = os.Unsetenv("SITE_TIMEZONE")
			}
		})

		date := newUTCDate()

		got := FormatDate(date, layout)

		assert.Equal(t, "2023-07-16 12:30", got)
	})

	t.Run("does not panic and keeps the original timezone on invalid timezone", func(t *testing.T) {
		t.Setenv("SITE_TIMEZONE", "Invalid/Timezone")

		date := newUTCDate()
		expected := date.Format(layout)

		var got string
		assert.NotPanics(t, func() {
			got = FormatDate(date, layout)
		}, "an invalid SITE_TIMEZONE should never panic")

		assert.Equal(t, expected, got,
			"an invalid timezone should format the date on its original timezone")
	})

}
