package metrics

const (
	// LabelMethod is the Prometheus label name for HTTP method.
	LabelMethod = "method"

	// LabelStatusCode is the Prometheus label name for HTTP status codes.
	LabelStatusCode = "code"

	// LabelStatus is the Prometheus label name for the status of a process
	// such as "success" or "error".
	LabelStatus = "status"

	// LabelService is the Prometheus label name for AWS API names.
	LabelService = "service"

	// LabelOperation is the Prometheus label name for operations within
	// an AWS API.
	LabelOperation = "operation"

	// LabelOperation is the Prometheus label name for the AWS region label.
	LabelRegion = "region"
)
