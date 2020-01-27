package drainer

// b is a quick and dirty map type for specifying JSON bodies.
type b map[string]interface{}

func strPtr(s string) *string {
	return &s
}
