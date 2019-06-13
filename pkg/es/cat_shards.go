package es

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	elastic "github.com/olivere/elastic/v7"
	"github.com/olivere/elastic/v7/uritemplates"
)

// CatShardsService returns the list of shards plus some additional
// information about them.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cat-shards.html
// for details.
type CatShardsService struct {
	client        *elastic.Client
	pretty        bool
	index         string
	bytes         string // b, k, m, or g
	local         *bool
	masterTimeout string
	columns       []string
	sort          []string // list of columns for sort order
}

// NewCatShardsService creates a new CatShardsService.
func NewCatShardsService(client *elastic.Client) *CatShardsService {
	return &CatShardsService{
		client: client,
	}
}

// Limit response to shards of this index pattern (by default all indices are returned).
func (s *CatShardsService) Index(index string) *CatShardsService {
	s.index = index
	return s
}

// Bytes represents the unit in which to display byte values.
// Valid values are: "b", "k", "m", or "g".
func (s *CatShardsService) Bytes(bytes string) *CatShardsService {
	s.bytes = bytes
	return s
}

// Local indicates to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *CatShardsService) Local(local bool) *CatShardsService {
	s.local = &local
	return s
}

// MasterTimeout is the explicit operation timeout for connection to master node.
func (s *CatShardsService) MasterTimeout(masterTimeout string) *CatShardsService {
	s.masterTimeout = masterTimeout
	return s
}

// Columns to return in the response.
// To get a list of all possible columns to return, run the following command
// in your terminal:
//
// Example:
//   curl 'http://localhost:9200/_cat/shards?help'
//
// Please use the long names for columns (i.e. `completion.size`) for JSON unmarshalling
// to work correctly.
// You can use Columns("*") to return all possible columns. That might take
// a little longer than the default set of columns.
func (s *CatShardsService) Columns(columns ...string) *CatShardsService {
	s.columns = columns
	return s
}

// Sort is a list of fields to sort by.
func (s *CatShardsService) Sort(fields ...string) *CatShardsService {
	s.sort = fields
	return s
}

// Pretty indicates that the JSON response be indented and human readable.
func (s *CatShardsService) Pretty(pretty bool) *CatShardsService {
	s.pretty = pretty
	return s
}

// buildURL builds the URL for the operation.
func (s *CatShardsService) buildURL() (string, url.Values, error) {
	// Build URL
	var (
		path string
		err  error
	)

	if s.index != "" {
		path, err = uritemplates.Expand("/_cat/shards/{index}", map[string]string{
			"index": s.index,
		})
	} else {
		path = "/_cat/shards"
	}
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{
		"format": []string{"json"}, // always returns as JSON
	}
	if s.pretty {
		params.Set("pretty", "true")
	}
	if s.bytes != "" {
		params.Set("bytes", s.bytes)
	}
	if v := s.local; v != nil {
		params.Set("local", fmt.Sprint(*v))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if len(s.columns) > 0 {
		params.Set("h", strings.Join(s.columns, ","))
	}
	if len(s.sort) > 0 {
		params.Set("s", strings.Join(s.sort, ","))
	}
	return path, params, nil
}

// Do executes the operation.
func (s *CatShardsService) Do(ctx context.Context) (CatShardsResponse, error) {
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
	var ret CatShardsResponse
	if err := (&elastic.DefaultDecoder{}).Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a get request.

// CatShardsResponse is the outcome of CatShardsService.Do.
type CatShardsResponse []CatShardsResponseRow

// CatShardsResponseRow specifies the data returned for one shard
// of a CatShardsResponse. Notice that not all of these fields might
// be filled; that depends on the number of columns chose in the
// request (see CatShardsService.Columns).
type CatShardsResponseRow struct {
	CompletionSize                 string `json:"completion.size"`                // size of completion
	Docs                           int    `json:"docs,string"`                    // number of docs in shard
	FieldDataEvictions             string `json:"fielddata.evictions"`            // fielddata evictions
	FieldDataMemorySize            string `json:"fielddata.memory_size"`          // used fielddata cache
	FlushTotal                     int    `json:"flush.total,string"`             // number of flushes
	FlushTotalTime                 string `json:"flush.total_time"`               // time spent in flush
	GetCurrent                     int    `json:"get.current,string"`             // number of current get ops
	GetExistsTime                  string `json:"get.exists_time"`                // time spent in successful gets
	GetExistsTotal                 int    `json:"get.exists_total,string"`        // number of successful gets
	GetMissingTime                 string `json:"get.missing_time"`               // time spent in failed gets
	GetMissingTotal                int    `json:"get.missing_total,string"`       // number of failed gets
	GetTime                        string `json:"get.time"`                       // time spent in get
	GetTotal                       int    `json:"get.total,string"`               // number of get ops
	ID                             string `json:"id"`                             // unique id of node where it lives
	Index                          string `json:"index"`                          // index name
	IndexingDeleteCurrent          int    `json:"indexing.delete_current,string"` // number of current deletions
	IndexingDeleteTotal            int    `json:"indexing.delete_total,string"`   // number of delete ops
	IndexingDeleteTime             string `json:"indexing.delete_time"`           // time spent in deletions
	IndexingIndexCurrent           int    `json:"indexing.index_current,string"`  // number of current indexing ops
	IndexingIndexFailed            int    `json:"indexing.index_failed,string"`   // number of failed indexing ops
	IndexingIndexTime              string `json:"indexing.index_time"`            // time spent in indexing
	IndexingIndexTotal             int    `json:"indexing.index_total,string"`    // number of indexing ops
	IP                             string `json:"ip"`                             // ip of node where it lives
	MergesCurrent                  int    `json:"merges.current,string"`          // number of current merges
	MergesCurrentDocs              int    `json:"merges.current_docs,string"`     // number of current merging docs
	MergesCurrentSize              string `json:"merges.current_size"`            // size of current merges
	MergesTotal                    int    `json:"merges.total,string"`            // number of completed merge ops
	MergesTotalDocs                int    `json:"merges.total_docs,string"`       // docs merged
	MergesTotalSize                string `json:"merges.total_size"`              // size merged
	MergesTotalTime                string `json:"merges.total_time"`              // time spent in merges
	Node                           string `json:"node"`                           // name of node where it lives
	PrimaryOrReplica               string `json:"prirep"`                         // primary ("p") or replica ("r")
	QueryCacheEvictions            int    `json:"query_cache.evictions,string"`   // query cache evictions
	QueryCacheMemorySize           string `json:"query_cache.memory_size"`        // used query cache
	RecoverySourceType             string `json:"recoverysource.type"`            // recovery source type
	RefreshListeners               int    `json:"refresh.listeners,string"`       // number of pending refresh listeners
	RefreshTime                    string `json:"refresh.time"`                   // time spent in refreshes
	RefreshTotal                   int    `json:"refresh.total,string"`           // total refreshes
	SearchFetchCurrent             int    `json:"search.fetch_current,string"`    // current fetch phase ops
	SearchFetchTime                string `json:"search.fetch_time"`              // time spent in fetch phase
	SearchFetchTotal               int    `json:"search.fetch_total,string"`      // total fetch ops
	SearchOpenContexts             int    `json:"search.open_contexts,string"`    // open search contexts
	SearchQueryCurrent             int    `json:"search.query_current,string"`    // current query phase ops
	SearchQueryTime                string `json:"search.query_time"`              // time spent in query phase
	SearchQueryTotal               int    `json:"search.query_total,string"`      // total query phase ops
	SearchScrollCurrent            int    `json:"search.scroll_current,string"`   // open scroll contexts
	SearchScrollTime               string `json:"search.scroll_time"`             // time scroll contexts held open
	SearchScrollTotal              int    `json:"search.scroll_total,string"`     // completed scroll contexts
	SegmentsCount                  int    `json:"segments.count,string"`          // number of segments
	SegmentsFixedBitsetMemory      string `json:"segments.fixed_bitset_memory"`   // memory used by fixed bit sets for nested object field types and type filters for types referred in _parent fields
	SegmentsIndexWriterMemory      string `json:"segments.index_writer_memory"`   // memory used by index writer
	SegmentsMemory                 string `json:"segments.memory"`                // memory used by segments
	SegmentsVersionMapMemory       string `json:"segments.version_map_memory"`    // memory used by version map
	SequenceNumberGlobalCheckpoint string `json:"seq_no.global_checkpoint"`       // global checkpoint
	SequenceNumberLocalCheckpoint  string `json:"seq_no.local_checkpoint"`        // local checkpoint
	SequenceNumberMax              string `json:"seq_no.max"`                     // max sequence number
	Shard                          string `json:"shard"`                          // shard name
	State                          string `json:"state"`                          // shard state
	Store                          string `json:"store"`                          // store size of shard (how much disk it uses)
	SyncID                         string `json:"sync_id"`                        // sync id
	UnassignedAt                   string `json:"unassigned.at"`                  // time shard became unassigned (UTC)
	UnassignedDeatils              string `json:"unassigned.details"`             // additional details as to why the shard became unassigned
	UnassignedFor                  string `json:"unassigned.for"`                 // time has been unassigned
	UnassignedReason               string `json:"unassigned.reason"`              // reason shard is unassigned (https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cat-shards.html#reason-unassigned)
	WarmerCurrent                  int    `json:"warmer.current,string"`          // current warmer ops
	WarmerTotal                    int    `json:"warmer.total,string"`            // total warmer ops
	WarmerTotalTime                string `json:"warmer.total_time"`              // time spent in warmers
}
