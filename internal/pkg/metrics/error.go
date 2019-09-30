package metrics

import (
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
)

// ElasticsearchStatusCode returns the HTTP status code from an Elasticsearch client error as a string.
// It's used for labels in Prometheus metrics. The err parameter can be of
// type *http.Response, *elastic.Error, elastic.Error, or int (indicating the HTTP status code).
// Returns 0 if err is not one of those types.
// Based on elastic.IsStatusCode (https://github.com/olivere/elastic/blob/release-branch.v7/errors.go#L151)
func ElasticsearchStatusCode(err interface{}) string {
	var code int
	switch e := err.(type) {
	case *http.Response:
		code = e.StatusCode
	case *elastic.Error:
		code = e.Status
	case elastic.Error:
		code = e.Status
	case int:
		code = e
	}
	if code == 0 {
		return ""
	}
	return strconv.Itoa(code)
}

// AWSStatusCode returns the HTTP status code from an AWS client error as a string.
// It's used for labels in Prometheus metrics. The err parameter can be of
// type *http.Response, awserr.RequestFailure, awserr.BatchedErrors (containing at least one awserr.RequestFailure),
// or int (indicating the HTTP status code).
// Returns 0 if err is not one of those types.
func AWSStatusCode(err interface{}) string {
	var code int
	switch e := err.(type) {
	case *http.Response:
		code = e.StatusCode
	case awserr.RequestFailure:
		code = e.StatusCode()
	case awserr.BatchedErrors:
		for _, berr := range e.OrigErrs() {
			if rerr, ok := berr.(awserr.RequestFailure); ok {
				code = rerr.StatusCode()
				break
			}
		}
	case int:
		code = e
	}
	if code == 0 {
		return ""
	}
	return strconv.Itoa(code)
}
