package es

import (
	"net/http"
	"testing"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/stretchr/testify/assert"    // Test assertions e.g. equality.
	gock "gopkg.in/h2non/gock.v1"           // HTTP request mocking.

	"github.com/mintel/elasticsearch-asg/internal/pkg/testutil" // Testing utilities.
)

func TestCatShardsService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			MatchParam("h", `\*`).
			Reply(http.StatusOK).
			BodyString(testutil.LoadTestData("cat_shards.json"))

		want := CatShardsResponse{
			CatShardsResponseRow{
				CompletionSize:                 strPtr("0b"),
				Docs:                           intPtr(0),
				FieldDataEvictions:             intPtr(0),
				FieldDataMemorySize:            strPtr("0b"),
				FlushTotal:                     intPtr(0),
				FlushTotalTime:                 strPtr("0s"),
				GetCurrent:                     intPtr(0),
				GetExistsTime:                  strPtr("0s"),
				GetExistsTotal:                 intPtr(0),
				GetMissingTime:                 strPtr("0s"),
				GetMissingTotal:                intPtr(0),
				GetTime:                        strPtr("0s"),
				GetTotal:                       intPtr(0),
				ID:                             strPtr("oWKmRBI8ToKc7qAibG7dRw"),
				Index:                          "twitter",
				IndexingDeleteCurrent:          intPtr(0),
				IndexingDeleteTime:             strPtr("0s"),
				IndexingDeleteTotal:            intPtr(0),
				IndexingIndexCurrent:           intPtr(0),
				IndexingIndexFailed:            intPtr(0),
				IndexingIndexTime:              strPtr("0s"),
				IndexingIndexTotal:             intPtr(0),
				IP:                             strPtr("10.20.0.2"),
				MergesCurrent:                  intPtr(0),
				MergesCurrentDocs:              intPtr(0),
				MergesCurrentSize:              strPtr("0b"),
				MergesTotal:                    intPtr(0),
				MergesTotalDocs:                intPtr(0),
				MergesTotalSize:                strPtr("0b"),
				MergesTotalTime:                strPtr("0s"),
				Node:                           strPtr("9426521fca1a"),
				PrimaryOrReplica:               "p",
				QueryCacheEvictions:            intPtr(0),
				QueryCacheMemorySize:           strPtr("0b"),
				RecoverySourceType:             nil,
				RefreshListeners:               intPtr(0),
				RefreshTime:                    strPtr("0s"),
				RefreshTotal:                   intPtr(2),
				SearchFetchCurrent:             intPtr(0),
				SearchFetchTime:                strPtr("0s"),
				SearchFetchTotal:               intPtr(0),
				SearchOpenContexts:             intPtr(0),
				SearchQueryCurrent:             intPtr(0),
				SearchQueryTime:                strPtr("0s"),
				SearchQueryTotal:               intPtr(0),
				SearchScrollCurrent:            intPtr(0),
				SearchScrollTime:               strPtr("0s"),
				SearchScrollTotal:              intPtr(0),
				SegmentsCount:                  intPtr(0),
				SegmentsFixedBitsetMemory:      strPtr("0b"),
				SegmentsIndexWriterMemory:      strPtr("0b"),
				SegmentsMemory:                 strPtr("0b"),
				SegmentsVersionMapMemory:       strPtr("0b"),
				SequenceNumberGlobalCheckpoint: strPtr("-1"),
				SequenceNumberLocalCheckpoint:  strPtr("-1"),
				SequenceNumberMax:              strPtr("-1"),
				Shard:                          "0",
				State:                          "STARTED",
				Store:                          strPtr("230b"),
				SyncID:                         nil,
				UnassignedAt:                   nil,
				UnassignedDeatils:              nil,
				UnassignedFor:                  nil,
				UnassignedReason:               nil,
				WarmerCurrent:                  intPtr(0),
				WarmerTotal:                    intPtr(1),
				WarmerTotalTime:                strPtr("1ms"),
			},
			CatShardsResponseRow{
				CompletionSize:                 nil,
				Docs:                           nil,
				FieldDataEvictions:             nil,
				FieldDataMemorySize:            nil,
				FlushTotal:                     nil,
				FlushTotalTime:                 nil,
				GetCurrent:                     nil,
				GetExistsTime:                  nil,
				GetExistsTotal:                 nil,
				GetMissingTime:                 nil,
				GetMissingTotal:                nil,
				GetTime:                        nil,
				GetTotal:                       nil,
				ID:                             nil,
				Index:                          "twitter",
				IndexingDeleteCurrent:          nil,
				IndexingDeleteTime:             nil,
				IndexingDeleteTotal:            nil,
				IndexingIndexCurrent:           nil,
				IndexingIndexFailed:            nil,
				IndexingIndexTime:              nil,
				IndexingIndexTotal:             nil,
				IP:                             nil,
				MergesCurrent:                  nil,
				MergesCurrentDocs:              nil,
				MergesCurrentSize:              nil,
				MergesTotal:                    nil,
				MergesTotalDocs:                nil,
				MergesTotalSize:                nil,
				MergesTotalTime:                nil,
				Node:                           nil,
				PrimaryOrReplica:               "r",
				QueryCacheEvictions:            nil,
				QueryCacheMemorySize:           nil,
				RecoverySourceType:             strPtr("peer"),
				RefreshListeners:               nil,
				RefreshTime:                    nil,
				RefreshTotal:                   nil,
				SearchFetchCurrent:             nil,
				SearchFetchTime:                nil,
				SearchFetchTotal:               nil,
				SearchOpenContexts:             nil,
				SearchQueryCurrent:             nil,
				SearchQueryTime:                nil,
				SearchQueryTotal:               nil,
				SearchScrollCurrent:            nil,
				SearchScrollTime:               nil,
				SearchScrollTotal:              nil,
				SegmentsCount:                  nil,
				SegmentsFixedBitsetMemory:      nil,
				SegmentsIndexWriterMemory:      nil,
				SegmentsMemory:                 nil,
				SegmentsVersionMapMemory:       nil,
				SequenceNumberGlobalCheckpoint: nil,
				SequenceNumberLocalCheckpoint:  nil,
				SequenceNumberMax:              nil,
				Shard:                          "0",
				State:                          "UNASSIGNED",
				Store:                          nil,
				SyncID:                         nil,
				UnassignedAt:                   timePtr(time.Date(2019, time.September, 30, 19, 03, 02, int(514*time.Millisecond), time.UTC)),
				UnassignedDeatils:              nil,
				UnassignedFor:                  strPtr("27.1s"),
				UnassignedReason:               strPtr("INDEX_CREATED"),
				WarmerCurrent:                  nil,
				WarmerTotal:                    nil,
				WarmerTotalTime:                nil,
			},
		}

		resp, err := NewCatShardsService(client).Columns("*").Do(ctx)
		assert.NoError(t, err)
		assert.Equal(t, want, resp)
		assert.Condition(t, gock.IsDone)
	})

	t.Run("error", func(t *testing.T) {
		ctx, _, teardown := testutil.ClientTestSetup(t)
		defer teardown()
		defer gock.CleanUnmatchedRequest()
		client, err := elastic.NewSimpleClient()
		if err != nil {
			panic(err)
		}

		gock.New(elastic.DefaultURL).
			Get("/_cat/shards").
			Reply(http.StatusInternalServerError).
			BodyString(http.StatusText(http.StatusInternalServerError))

		_, err = NewCatShardsService(client).Do(ctx)
		assert.Error(t, err)
		assert.Condition(t, gock.IsDone)
	})

}
