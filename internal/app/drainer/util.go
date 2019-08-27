package drainer

import "strings"

func topicKey(parts ...string) string {
	return strings.Join(parts, ":")
}
