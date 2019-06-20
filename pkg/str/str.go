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

// Uniq returns a new slice containing only unique strings.
// The order of the return value is undefined.
func Uniq(strs ...string) []string {
	m := make(map[string]struct{}, len(strs))
	for _, s := range strs {
		m[s] = struct{}{}
	}
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	return out
}
