package cmd

import (
	"net/url"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
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

		// max backoff duration.
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

func (f *ElasticsearchFlags) ElasticsearchConfig(opts ...elastic.ClientOptionFunc) []elastic.ClientOptionFunc {
	urls := make([]string, len(f.URLs))
	for i, u := range f.URLs {
		urls[i] = u.String()
	}
	opts = append(opts, elastic.SetURL(urls...))

	if f.Retry.Max > 0 {
		backoff := elastic.NewExponentialBackoff(f.Retry.Init, f.Retry.Max)
		retrier := elastic.NewBackoffRetrier(backoff)
		opts = append(opts, elastic.SetRetrier(retrier))
	}

	return opts
}
