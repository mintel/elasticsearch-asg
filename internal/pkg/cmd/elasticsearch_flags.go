package cmd

import (
	"net/url"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.

	"github.com/mintel/elasticsearch-asg/pkg/es" // Extensions to the Elasticsearch client.
)

// ElasticsearchFlags represents a base set of flags for
// connecting to Elasticsearch.
type ElasticsearchFlags struct {
	// URL(s) of Elasticsearch nodes to connect to.
	URLs []*url.URL

	// Exponential backoff retries flags.
	Retry struct {
		// Initial backoff duration.
		Init time.Duration

		// Max backoff duration.
		Max time.Duration
	}
}

// NewElasticsearchFlags returns a new BaseFlags.
func NewElasticsearchFlags(app Flagger, retryInit, retryMax time.Duration) *ElasticsearchFlags {
	var f ElasticsearchFlags

	app.Flag("elasticsearch.url", "URL(s) of Elasticsearch.").
		Short('e').
		Default(elastic.DefaultURL).
		URLListVar(&f.URLs)

	app.Flag("elasticsearch.retry.init", "Initial duration of Elasticsearch exponential backoff retries.").
		Hidden().
		Default(retryInit.String()).
		DurationVar(&f.Retry.Init)

	app.Flag("elasticsearch.retry.max", "Max duration of Elasticsearch exponential backoff retries.").
		Hidden().
		Default(retryMax.String()).
		DurationVar(&f.Retry.Init)

	return &f
}

// NewElasticsearchClient returns a new Elasticsearch client
// configured with the URL and retry flag values, plus any other options
// passed in.
func (f *ElasticsearchFlags) NewElasticsearchClient(options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	urls := make([]string, len(f.URLs))
	for i, u := range f.URLs {
		urls[i] = u.String()
	}
	options = append(options, elastic.SetURL(urls...))
	return es.DialRetry(f.Retry.Init, f.Retry.Max, options...)
}
