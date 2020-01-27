package es

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/tidwall/gjson"              // Dynamic JSON parsing.
)

// ClusterGetSettingsService gets the settings of an Elasticsearch cluster.
// I can't believe github.com/olivere/elastic doesn't have this but aparently not.
type ClusterGetSettingsService struct {
	client          *elastic.Client
	includeDefaults bool
	pretty          bool
	human           *bool
	filterPath      []string
}

// NewClusterGetSettingsService returns a new ClusterGetSettingsService.
func NewClusterGetSettingsService(client *elastic.Client) *ClusterGetSettingsService {
	return &ClusterGetSettingsService{
		client:     client,
		filterPath: make([]string, 0),
	}
}

func (s *ClusterGetSettingsService) buildURL() (string, url.Values, error) {
	var err error
	path := "/_cluster/settings"

	params := url.Values{}
	if s.pretty {
		params.Set("pretty", "true")
	}
	if s.includeDefaults {
		params.Set("include_defaults", "true")
	}
	if len(s.filterPath) > 0 {
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.human != nil {
		params.Set("human", fmt.Sprintf("%v", *s.human))
	}

	return path, params, err
}

// Defaults indicates if Elasticsearch should include default settings values in the response.
func (s *ClusterGetSettingsService) Defaults(include bool) *ClusterGetSettingsService {
	s.includeDefaults = include
	return s
}

// FilterPath allows reducing the response, a mechanism known as
// response filtering and described here:
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/common-options.html#common-options-response-filtering.
func (s *ClusterGetSettingsService) FilterPath(filterPath ...string) *ClusterGetSettingsService {
	s.filterPath = append(s.filterPath, filterPath...)
	return s
}

// Pretty enables the caller to indent the JSON output.
func (s *ClusterGetSettingsService) Pretty(pretty bool) *ClusterGetSettingsService {
	s.pretty = pretty
	return s
}

// Human indicates whether to return version and creation date values
// in human-readable format (default: false).
func (s *ClusterGetSettingsService) Human(human bool) *ClusterGetSettingsService {
	s.human = &human
	return s
}

// Validate checks if the operation is valid.
// Just cargo-culting from elastic here.
func (s *ClusterGetSettingsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *ClusterGetSettingsService) Do(ctx context.Context) (*ClusterGetSettingsResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, elastic.PerformRequestOptions{
		Method: "GET",
		Path:   path,
		Params: params,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	response, err := res.Body.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if !gjson.Valid(string(response)) {
		return nil, errors.New("invalid json")
	}
	result := gjson.ParseBytes(response)
	persistent := result.Get("persistent")
	transient := result.Get("transient")
	ret := &ClusterGetSettingsResponse{
		Persistent: &persistent,
		Transient:  &transient,
	}
	if s.includeDefaults {
		defaults := result.Get("defaults")
		ret.Defaults = &defaults
	}
	return ret, nil
}

// ClusterGetSettingsResponse represents the response from the Elasticsearch
// `GET /_cluster/settings` API.
type ClusterGetSettingsResponse struct {
	Persistent *gjson.Result
	Transient  *gjson.Result
	Defaults   *gjson.Result
}
