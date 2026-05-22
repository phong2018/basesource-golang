package helper

import "time"

const DateTimeFormat = "2006-01-02 15:04:05"

func FormatTime(t time.Time) string {
	return t.Format(DateTimeFormat)
}
