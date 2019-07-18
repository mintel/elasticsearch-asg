package es

import (
	"context"
	"fmt"
	"net/url"

	elastic "github.com/olivere/elastic/v7"
)

// ClusterPostVotingConfigExclusion removes all voting configuration exclusions,
// allowing any node to return to the voting configuration in the future.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
type ClusterPostVotingConfigExclusion struct {
	client *elastic.Client

	node    string
	timeout string
}

// NewClusterPostVotingConfigExclusion returns a new ClusterPostVotingConfigExclusion.
func NewClusterPostVotingConfigExclusion(c *elastic.Client) *ClusterPostVotingConfigExclusion {
	return &ClusterPostVotingConfigExclusion{
		client: c,
	}
}

// Node sets the node(s) that should be excluded from voting configuration.
func (s *ClusterPostVotingConfigExclusion) Node(node string) *ClusterPostVotingConfigExclusion {
	s.node = node
	return s
}

// Timeout sets how long to wait for the system to auto-reconfigure the node out of the voting configuration.
// The default is 30 seconds.
func (s *ClusterPostVotingConfigExclusion) Timeout(timeout string) *ClusterPostVotingConfigExclusion {
	s.timeout = timeout
	return s
}

// Validate checks if the operation is valid.
func (s *ClusterPostVotingConfigExclusion) Validate() error {
	if s.node == "" {
		return fmt.Errorf("non-empty node required")
	}
	return nil
}

func (s *ClusterPostVotingConfigExclusion) buildURL() (string, url.Values, error) {
	var err error
	path := fmt.Sprintf("/_cluster/voting_config_exclusions/%s", s.node)

	params := url.Values{}
	if s.timeout != "" {
		params.Add("timeout", s.timeout)
	}

	return path, params, err
}

// Do executes the operation.
func (s *ClusterPostVotingConfigExclusion) Do(ctx context.Context) (*ClusterPostVotingConfigExclusionResponse, error) {
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
	_, err = s.client.PerformRequest(ctx, elastic.PerformRequestOptions{
		Method: "POST",
		Path:   path,
		Params: params,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(ClusterPostVotingConfigExclusionResponse)
	return ret, nil
}

// ClusterPostVotingConfigExclusionResponse represents the response from ClusterPostVotingConfigExclusion.
type ClusterPostVotingConfigExclusionResponse struct{}
