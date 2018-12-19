package wildcard

// Matchers is a slice of Matcher, matching any of the contained matchers.
type Matchers []*Matcher

// MatchAny returns true iff any of the matchers returns true.
func (m Matchers) MatchAny(s string) bool {
	for _, m := range m {
		if m.Match(s) {
			return true
		}
	}
	return false
}
