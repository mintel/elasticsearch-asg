package squeues

import (
	"fmt"
	"net/url"
	"strings"
)

// Region parses an SQS queue URL to return the AWS region its in.
func Region(URL string) (string, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return "", err
	}
	parts := strings.Split(u.Host, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("Invalid SQS hostname: '%s'", u.Host)
	}
	region := parts[1]
	return region, nil
}
