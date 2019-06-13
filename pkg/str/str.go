// Package str contains string utilities.
package str

// In returns true if string v is one of s strings.
func In(v string, s ...string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Uniq returns unique strings.
func Uniq(strs ...string) []string {
	m := make(map[string]bool, len(strs))
	for _, s := range strs {
		m[s] = true
	}
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	return out
}
