package elasticsearch

import (
	"context"

	elastic "github.com/olivere/elastic/v7"
)

// New returns both a new Command and Query.
func New(ctx context.Context, url string, options ...elastic.ClientOptionFunc) (*Command, *Query, error) {
	options = append(options, elastic.SetURL(url))
	client, err := elastic.DialContext(ctx, options...)
	if err != nil {
		return nil, nil, err
	}
	return NewCommand(client), NewQuery(client), nil
}
