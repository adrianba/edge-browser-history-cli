package edge

import (
	"fmt"
	"time"
)

// chromeEpochOffsetMicroseconds is the number of microseconds between the
// Chromium/Windows epoch (1601-01-01) and the Unix epoch (1970-01-01).
const chromeEpochOffsetMicroseconds int64 = 11644473600 * 1000000

// Valid Chrome timestamp range: 1601-01-01 to ~year 9999.
const (
	minChromeMicroseconds int64 = 0
	maxChromeMicroseconds int64 = 265046774399999999
)

// visitTimeLayout matches the .NET round-trip ("O") format with 7 fractional
// digits and a numeric UTC offset.
const visitTimeLayout = "2006-01-02T15:04:05.0000000-07:00"

// dateLayout is the local date format accepted on the command line.
const dateLayout = "2006-01-02"

// timeRangeToUTC converts a local date and optional local start/end times into
// an inclusive-start, exclusive-end UTC range. Times are interpreted in the
// machine local timezone (DST-aware).
func timeRangeToUTC(day time.Time, startTime, endTime string) (time.Time, time.Time, error) {
	year, month, dayOfMonth := day.Date()

	start := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.Local)
	if startTime != "" {
		h, m, s, err := parseTime(startTime)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		start = time.Date(year, month, dayOfMonth, h, m, s, 0, time.Local)
	}

	end := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.Local).AddDate(0, 0, 1)
	if endTime != "" {
		h, m, s, err := parseTime(endTime)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end = time.Date(year, month, dayOfMonth, h, m, s, 0, time.Local)
	}

	if !start.Before(end) {
		return time.Time{}, time.Time{}, newHistoryError("Invalid time range: --start-time must be earlier than --end-time.")
	}

	return start.UTC(), end.UTC(), nil
}

func parseTime(value string) (int, int, int, error) {
	if t, err := time.Parse("15:04:05", value); err == nil {
		return t.Hour(), t.Minute(), t.Second(), nil
	}
	if t, err := time.Parse("15:04", value); err == nil {
		return t.Hour(), t.Minute(), 0, nil
	}
	return 0, 0, 0, newHistoryError(fmt.Sprintf("Invalid time '%s'; expected HH:mm or HH:mm:ss.", value))
}

// timeToChromeMicroseconds converts a time to Chromium microseconds since the
// 1601 epoch.
func timeToChromeMicroseconds(t time.Time) int64 {
	return t.UTC().UnixMicro() + chromeEpochOffsetMicroseconds
}

// chromeMicrosecondsToLocal converts a Chromium timestamp to a local time.
// It returns ok=false if the timestamp is out of the valid range.
func chromeMicrosecondsToLocal(chromeMicroseconds int64) (time.Time, bool) {
	if chromeMicroseconds < minChromeMicroseconds || chromeMicroseconds > maxChromeMicroseconds {
		return time.Time{}, false
	}
	unixMicros := chromeMicroseconds - chromeEpochOffsetMicroseconds
	return time.UnixMicro(unixMicros).In(time.Local), true
}
