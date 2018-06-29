package apmstrings

// Truncate returns s truncated at n runes.
func Truncate(s string, n int) string {
	var j int
	for i := range s {
		if j == n {
			return s[:i]
		}
		j++
	}
	return s
}
