package es

import (
	"context"
	"time"

	elastic "github.com/olivere/elastic/v7"
)

// DialContextRetry returns a new Elasticsearch client that uses
// exponential backoff to retry in case of errors. More importantly, it
// uses retry/backoff for the initial connection to Elasticsearch,
// which the standard elastic.NewClient() func doesn't.
//
// If the max duration <= 0, a client without retry is returned.
// DialContextRetry won't retry on non-connection errors.
func DialContextRetry(ctx context.Context, init, max time.Duration, options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	if max <= 0 {
		return elastic.DialContext(ctx, options...)
	}
	backoff := elastic.NewExponentialBackoff(init, max)
	retrier := elastic.NewBackoffRetrier(backoff)
	options = append(options, elastic.SetRetrier(retrier))
	for i := 0; ; i++ {
		c, err := elastic.DialContext(ctx, options...)
		if err == nil {
			return c, nil
		}
		if !elastic.IsConnErr(err) {
			return nil, err
		}
		wait, goahead, _ := retrier.Retry(ctx, i, nil, nil, err)
		if !goahead {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// DialRetry returns a new Elasticsearch client that uses
// exponential backoff to retry in case of errors. More importantly, it
// uses retry/backoff for the initial connection to Elasticsearch,
// which the standard elastic.NewClient() func doesn't.
//
// If the max duration <= 0, a client without retry is returned.
// DialRetry won't retry on non-connection errors.
func DialRetry(init, max time.Duration, options ...elastic.ClientOptionFunc) (*elastic.Client, error) {
	return DialContextRetry(context.Background(), init, max, options...)
}
