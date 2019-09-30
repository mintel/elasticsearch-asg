package es

import (
	"context"
	"net/url"
	"strings"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesRecoveryService gets information about on-going and completed index
// shard recoveries.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-recovery.html
// for details.
type IndicesRecoveryService struct {
	client *elastic.Client

	index      []string
	detailed   *bool
	activeOnly *bool
}

// NewIndicesRecoveryService returns a new IndicesRecoveryService.
func NewIndicesRecoveryService(client *elastic.Client) *IndicesRecoveryService {
	return &IndicesRecoveryService{
		client: client,
	}
}

// Index limits the response to these indices.
// By default all indices are returned.
func (s *IndicesRecoveryService) Index(index ...string) *IndicesRecoveryService {
	s.index = append(s.index, index...)
	return s
}

// Detailed returns extra detail with the response, such as a
// list of physical files in recovery
func (s *IndicesRecoveryService) Detailed(detailed bool) *IndicesRecoveryService {
	s.detailed = &detailed
	return s
}

// ActiveOnly limits the response to only on-going recoveries.
func (s *IndicesRecoveryService) ActiveOnly(activeOnly bool) *IndicesRecoveryService {
	s.activeOnly = &activeOnly
	return s
}

// Validate checks if the operation is valid.
func (s *IndicesRecoveryService) Validate() error {
	return nil
}

func (s *IndicesRecoveryService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	if len(s.index) != 0 {
		path, err = uritemplates.Expand("/{index}/_recovery", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_recovery"
	}
	if err != nil {
		return "", url.Values{}, err
	}

	params := url.Values{}
	if s.detailed != nil {
		if *s.detailed {
			params.Add("detailed", "true")
		} else {
			params.Add("detailed", "false")
		}
	}

	if s.activeOnly != nil {
		if *s.activeOnly {
			params.Add("active_only", "true")
		} else {
			params.Add("active_only", "false")
		}
	}

	return path, params, err
}

// Do executes the operation.
func (s *IndicesRecoveryService) Do(ctx context.Context) (IndicesRecoveryResponse, error) {
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
	var ret IndicesRecoveryResponse
	if err := new(elastic.DefaultDecoder).Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesRecoveryResponse represents the response from IndicesRecoveryService.
// Keys are index names.
type IndicesRecoveryResponse map[string]IndicesRecoveryResponseIndex

// IndicesRecoveryResponseIndex represents the recovery of an index.
type IndicesRecoveryResponseIndex struct {
	Shards []*IndicesRecoveryResponseShard `json:"shards"`
}

// IndicesRecoveryResponseShard represents the recovery of a shard within
// an index.
type IndicesRecoveryResponseShard struct {
	ID                int                                     `json:"id"`
	Type              string                                  `json:"type"`  // One of: "store", "snapshot", "replica", "relocating"
	Stage             string                                  `json:"stage"` // One of: "init", "index", "start", "translog", "finalize", "done"
	Primary           bool                                    `json:"primary"`
	StartTime         *time.Time                              `json:"start_time"`
	StartTimeInMillis int64                                   `json:"start_time_in_millis"`
	StopTime          *time.Time                              `json:"stop_time"`
	StopTimeInMillis  int64                                   `json:"stop_time_in_millis"`
	TotalTime         string                                  `json:"total_time"` // e.g. 2.1s
	TotalTimeInMillis int64                                   `json:"total_time_in_millis"`
	Source            map[string]interface{}                  `json:"source"`
	Target            IndicesRecoveryResponseShardTarget      `json:"target"`
	Index             IndicesRecoveryResponseShardIndex       `json:"index"`
	Translog          IndicesRecoveryResponseShardTranslog    `json:"translog"`
	VerifyIndex       IndicesRecoveryResponseShardVerifyIndex `json:"verify_index"`
}

// IndicesRecoveryResponseShardTarget represents information about where
// a shard is being recovered to.
type IndicesRecoveryResponseShardTarget struct {
	ID               string `json:"id"`
	Host             string `json:"host"`
	TransportAddress string `json:"transport_address"`
	IP               string `json:"ip"`
	Name             string `json:"name"`
}

// IndicesRecoveryResponseShardIndex represents statistics about
// physical index recovery.
type IndicesRecoveryResponseShardIndex struct {
	Size                       IndicesRecoveryResponseShardIndexSize  `json:"size"`
	Files                      IndicesRecoveryResponseShardIndexFiles `json:"files"`
	TotalTime                  string                                 `json:"total_time"` // e.g. 2.1s
	TotalTimeInMillis          int64                                  `json:"total_time_in_millis"`
	SourceThrottleTime         string                                 `json:"source_throttle_time"` // e.g. 2.1s
	SourceThrottleTimeInMillis int64                                  `json:"source_throttle_time_in_millis"`
	TargetThrottleTime         string                                 `json:"target_throttle_time"` // e.g. 2.1s
	TargetThrottleTimeInMillis int64                                  `json:"target_throttle_time_in_millis"`
}

// IndicesRecoveryResponseShardIndexSize represents the size
// of a recovering shard.
type IndicesRecoveryResponseShardIndexSize struct {
	Total            string `json:"total"` // e.g. 2.1gb
	TotalInBytes     int64  `json:"total_in_bytes"`
	Reused           string `json:"reused"` // e.g. 2.1gb
	ReusedInBytes    int64  `json:"reused_in_bytes"`
	Recovered        string `json:"recovered"` // e.g. 2.1gb
	RecoveredInBytes int64  `json:"recovered_in_bytes"`
	Percent          string `json:"percent"` // e.g. 100.0%
}

// IndicesRecoveryResponseShardIndexFiles represents info about
// the recovery of individual files within a shard.
type IndicesRecoveryResponseShardIndexFiles struct {
	Total     int    `json:"total"`
	Reused    int    `json:"reused"`
	Recovered int64  `json:"recovered"`
	Percent   string `json:"percent"` // e.g. 100.0%
	Details   []struct {
		Name      string `json:"name"`
		Length    int64  `json:"length"`
		Recovered int64  `json:"recovered"`
	} `json:"details"`
}

// IndicesRecoveryResponseShardTranslog represents statistics
// about translog recovery.
type IndicesRecoveryResponseShardTranslog struct {
	Recovered         int    `json:"recovered"`
	Total             int    `json:"total"`
	Percent           string `json:"percent"` // e.g. 100.0%
	TotalOnStart      int    `json:"total_on_start"`
	TotalTime         string `json:"total_time"` // e.g. 2.1s
	TotalTimeInMillis int64  `json:"total_time_in_millis"`
}

// IndicesRecoveryResponseShardVerifyIndex represents the progress
// verifing an index recovery.
type IndicesRecoveryResponseShardVerifyIndex struct {
	CheckIndexTime         int    `json:"check_index_time"`
	CheckIndexTimeInMillis int64  `json:"check_index_time_in_millis"`
	TotalTime              string `json:"total_time"` // e.g. 2.1s
	TotalTimeInMillis      int64  `json:"total_time_in_millis"`
}
