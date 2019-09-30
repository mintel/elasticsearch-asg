package es

import (
	"context"
	"net/url"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
)

// ClusterDeleteVotingConfigExclusion removes all voting configuration exclusions,
// allowing any node to return to the voting configuration in the future.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/voting-config-exclusions.html
type ClusterDeleteVotingConfigExclusion struct {
	client *elastic.Client

	wait *bool
}

// NewClusterDeleteVotingConfigExclusion returns a new ClusterDeleteVotingConfigExclusion.
func NewClusterDeleteVotingConfigExclusion(c *elastic.Client) *ClusterDeleteVotingConfigExclusion {
	return &ClusterDeleteVotingConfigExclusion{
		client: c,
	}
}

// Wait (if true) for all the nodes with voting configuration exclusions to be removed from
// the cluster, and then remove the exclusions.
func (s *ClusterDeleteVotingConfigExclusion) Wait(wait bool) *ClusterDeleteVotingConfigExclusion {
	s.wait = &wait
	return s
}

// Validate checks if the operation is valid.
func (s *ClusterDeleteVotingConfigExclusion) Validate() error {
	return nil
}

func (s *ClusterDeleteVotingConfigExclusion) buildURL() (string, url.Values, error) {
	var err error
	path := "/_cluster/voting_config_exclusions"

	params := url.Values{}
	if s.wait != nil {
		if *s.wait {
			params.Add("wait_for_removal", "true")
		} else {
			params.Add("wait_for_removal", "false")
		}
	}

	return path, params, err
}

// Do executes the operation.
func (s *ClusterDeleteVotingConfigExclusion) Do(ctx context.Context) (*ClusterDeleteVotingConfigExclusionResponse, error) {
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
		Method: "DELETE",
		Path:   path,
		Params: params,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	return &ClusterDeleteVotingConfigExclusionResponse{}, nil
}

// ClusterDeleteVotingConfigExclusionResponse represents the response from ClusterDeleteVotingConfigExclusion.
type ClusterDeleteVotingConfigExclusionResponse struct{}
