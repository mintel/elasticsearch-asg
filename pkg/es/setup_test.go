package es

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
)

func prefix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// runElasticsearch runs an Elasticsearch Docker container, returning its handler
// and a client connected to it.
func runElasticsearch(t *testing.T) (*dockertest.Resource, *elastic.Client, error) {
	if testing.Short() {
		// Skip during short testing because running a Docker container
		// per test takes a while.
		t.Skipf("skipping during -short due to dependency on an Elasticsearch container")
	}

	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Docker: %s", err)
	}

	p := prefix(6)
	name := p + "-elasticsearch1"
	es, err := pool.RunWithOptions(&dockertest.RunOptions{
		Hostname:     name,
		Name:         name,
		Repository:   "docker.elastic.co/elasticsearch/elasticsearch-oss",
		Tag:          "7.2.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: []string{
			"cluster.name=elasticsearch",
			"bootstrap.memory_lock=true",
			"discovery.type=single-node",
			"ES_JAVA_OPTS=-Xms256m -Xmx256m",
		},
	}, func(hc *docker.HostConfig) {
		hc.AutoRemove = true
		hc.Ulimits = []docker.ULimit{
			docker.ULimit{
				Name: "nproc",
				Soft: 65536,
				Hard: 65536,
			},
			docker.ULimit{
				Name: "nofile",
				Soft: 65536,
				Hard: 65536,
			},
			docker.ULimit{
				Name: "memlock",
				Soft: -1,
				Hard: -1,
			},
		}
	})
	if err != nil {
		return nil, nil, err
	}

	var client *elastic.Client
	if err := pool.Retry(func() error {
		u := "http://" + es.GetHostPort("9200/tcp")
		if client, err = elastic.NewSimpleClient(elastic.SetURL(u)); err != nil {
			return err
		}
		_, _, err = client.Ping(u).Do(context.Background())
		return err
	}); err != nil {
		es.Close()
		return nil, nil, fmt.Errorf("error waiting for Elasticsearch container: %s", err)
	}

	return es, client, nil
}
