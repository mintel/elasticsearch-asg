package elasticsearch

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality
	gock "gopkg.in/h2non/gock.v1"           // HTTP endpoint mocking
)

const (
	node1Name          = "foo"
	node2Name          = "bar"
	sortedNodeNameList = "bar,foo"
)


func TestCommand_Drain(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	cmd := NewCommand(esClient)

	// Drain a node.
	t.Run("drain-first", func(t *testing.T) {
		gock.New(u).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})
		gock.New(u).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node1Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		err = cmd.Drain(context.Background(), node1Name)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

	// Draining the same node again shouldn't need to PUT any settings.
	t.Run("drain-first-again", func(t *testing.T) {
		gock.New(u).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		err := cmd.Drain(context.Background(), node1Name)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

	// Draining a second node results in a comma-separated list in sorted order.
	t.Run("drain-second", func(t *testing.T) {
		gock.New(u).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node1Name}}}}}})
		gock.New(u).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": sortedNodeNameList}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
		err := cmd.Drain(context.Background(), node2Name)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})
}

func TestCommand_Undrain(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	s := NewCommand(esClient)

	// Undrain a node.
	t.Run("undrain-first", func(t *testing.T) {
		gock.New(u).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": sortedNodeNameList}}}}}})
		gock.New(u).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": node2Name}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
		err = s.Undrain(context.Background(), node1Name)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

	// Undrain last node.
	t.Run("undrain-last", func(t *testing.T) {
		gock.New(u).
			Get("/_cluster/settings").
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{"cluster": b{"routing": b{"allocation": b{"exclude": b{"_name": node2Name}}}}}})
		gock.New(u).
			Put("/_cluster/settings").
			JSON(b{"transient": b{"cluster.routing.allocation.exclude._name": nil}}).
			Reply(http.StatusOK).
			JSON(b{"persistent": b{}, "transient": b{}})
		err := s.Undrain(context.Background(), node2Name)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})
}

func TestCommand_EnsureSnapshotRepo(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	cmd := NewCommand(esClient)

	const (
		repoName = "myrepo"
		repoType = "s3"
	)
	repoSettings := map[string]string{
		"bucket": "foobar",
	}

	t.Run("not-exist", func(t *testing.T) {
		gock.New(u).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusNotFound).
			JSON(b{
				"error": b{
					"reason": fmt.Sprintf("[%s] missing", repoName),
					"root_cause": []b{
						b{
							"reason": fmt.Sprintf("[%s] missing", repoName),
							"type":   "repository_missing_exception",
						},
					},
					"type": "repository_missing_exception",
				},
				"status": 404,
			})
		gock.New(u).
			Put(fmt.Sprintf("/_snapshot/%s", repoName)).
			JSON(b{"type": repoType, "settings": repoSettings}).
			Reply(http.StatusOK).
			JSON(b{"acknowledged": true})
		err := cmd.EnsureSnapshotRepo(context.Background(), repoName, repoType, repoSettings)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

	t.Run("does-exist", func(t *testing.T) {
		gock.New(u).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusOK).
			JSON(b{
				repoName: b{
					"type":     repoType,
					"settings": repoSettings,
				},
			})
		err := cmd.EnsureSnapshotRepo(context.Background(), repoName, repoType, repoSettings)
		assert.NoError(t, err)
		assert.True(t, gock.IsDone())
	})

	t.Run("wrong-type", func(t *testing.T) {
		gock.New(u).
			Get(fmt.Sprintf("/_snapshot/%s", repoName)).
			Reply(http.StatusOK).
			JSON(b{
				repoName: b{
					"type":     "wrongtype",
					"settings": repoSettings,
				},
			})
		err := cmd.EnsureSnapshotRepo(context.Background(), repoName, repoType, repoSettings)
		assert.Error(t, err)
		assert.True(t, gock.IsDone())
	})
}

func TestCommand_CreateSnapshot(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	cmd := NewCommand(esClient)

	const (
		repoName     = "myrepo"
		format       = "analytics-20060102t150405"
		snapshotName = "analytics-20190903t040001"
	)
	now := time.Date(2019, 9, 3, 4, 0, 1, 0, time.UTC)

	gock.New(u).
		Put(fmt.Sprintf("/_snapshot/%s/%s", repoName, snapshotName)).
		MatchParam("wait_for_completion", "true").
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err = cmd.CreateSnapshot(context.Background(), repoName, format, now)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}

func TestCommand_DeleteSnapshot(t *testing.T) {
	gock.Intercept()
	defer gock.OffAll()
	// gock.Observe(gock.DumpRequest) // Log HTTP requests during test.

	u, teardown := setup(t)
	defer teardown()

	esClient, err := elastic.NewSimpleClient(elastic.SetURL(u))
	if err != nil {
		t.Fatalf("couldn't create elastic client: %s", err)
	}
	cmd := NewCommand(esClient)

	const (
		repoName     = "myrepo"
		snapshotName = "analytics-20190903t040001"
	)

	gock.New(u).
		Delete(fmt.Sprintf("/_snapshot/%s/%s", repoName, snapshotName)).
		Reply(http.StatusOK).
		JSON(b{"acknowledged": true})
	err = cmd.DeleteSnapshot(context.Background(), repoName, snapshotName)
	assert.NoError(t, err)
	assert.True(t, gock.IsDone())
}
