package es

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/tidwall/gjson"              // Dynamic JSON parsing.
)

// ClusterPutSettingsService updates the settings of an Elasticsearch cluster.
// I can't believe github.com/olivere/elastic doesn't have this but aparently not.
type ClusterPutSettingsService struct {
	client        *elastic.Client
	pretty        bool
	flatSettings  *bool
	masterTimeout string
	bodyJSON      interface{}
	bodyString    string
	transient     map[string]interface{}
	persistent    map[string]interface{}
}

// NewClusterPutSettingsService returns a new ClusterPutSettingsService.
func NewClusterPutSettingsService(client *elastic.Client) *ClusterPutSettingsService {
	return &ClusterPutSettingsService{
		client:     client,
		transient:  make(map[string]interface{}),
		persistent: make(map[string]interface{}),
	}
}

// Transient adds a transient settings to the request.
func (s *ClusterPutSettingsService) Transient(setting string, value interface{}) *ClusterPutSettingsService {
	s.transient[setting] = value
	return s
}

// Persistent adds a persistent settings to the request.
func (s *ClusterPutSettingsService) Persistent(setting string, value interface{}) *ClusterPutSettingsService {
	s.persistent[setting] = value
	return s
}

// FlatSettings indicates whether to return settings in flat format (default: false).
func (s *ClusterPutSettingsService) FlatSettings(flatSettings bool) *ClusterPutSettingsService {
	s.flatSettings = &flatSettings
	return s
}

// MasterTimeout is the timeout for connection to master.
func (s *ClusterPutSettingsService) MasterTimeout(masterTimeout string) *ClusterPutSettingsService {
	s.masterTimeout = masterTimeout
	return s
}

// Pretty indicates that the JSON response be indented and human readable.
func (s *ClusterPutSettingsService) Pretty(pretty bool) *ClusterPutSettingsService {
	s.pretty = pretty
	return s
}

// BodyJSON is documented as: The index settings to be updated.
func (s *ClusterPutSettingsService) BodyJSON(body interface{}) *ClusterPutSettingsService {
	s.bodyJSON = body
	return s
}

// BodyString is documented as: The index settings to be updated.
func (s *ClusterPutSettingsService) BodyString(body string) *ClusterPutSettingsService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *ClusterPutSettingsService) buildURL() (string, url.Values, error) {
	// Build URL
	path := "/_cluster/settings"

	// Add query string parameters
	params := url.Values{}
	if s.pretty {
		params.Set("pretty", "true")
	}
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *ClusterPutSettingsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *ClusterPutSettingsService) Do(ctx context.Context) (*ClusterPutSettingsResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	var body interface{}
	if s.bodyJSON != nil {
		body = s.bodyJSON
	} else if s.bodyString != "" {
		body = s.bodyString
	} else {
		body = map[string]interface{}{
			"persistent": s.persistent,
			"transient":  s.transient,
		}
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, elastic.PerformRequestOptions{
		Method: "PUT",
		Path:   path,
		Params: params,
		Body:   body,
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
	ret := &ClusterPutSettingsResponse{
		Persistent: &persistent,
		Transient:  &transient,
	}
	return ret, nil
}

// ClusterPutSettingsResponse represents the response from the Elasticsearch
// `PUT /_cluster/settings` API. It contains the new values of the changed settings.
type ClusterPutSettingsResponse struct {
	// Persistent hold the Elasticsearch settings that persist between cluster restarts.
	Persistent *gjson.Result

	// Transient hold the Elasticsearch settings that do not persist between cluster restarts.
	Transient *gjson.Result
}
