// Package elasticsearch implements CQRS-style Command/Query services that
// encapsulate the Elasticsearch interactions that happen in elasticsearch-asg.
// Interacting with Elasticsearch through these common interfaces means
// that Prometheus metrics for Elasticsearch requests only need to be added
// in only one place.
package elasticsearch
