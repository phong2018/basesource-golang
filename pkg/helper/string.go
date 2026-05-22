package helper

func IsEmpty(s string) bool { return s == "" }

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
