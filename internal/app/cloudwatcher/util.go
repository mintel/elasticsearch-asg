package cloudwatcher

import (
	"bytes"
	"compress/gzip"
	"io"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	elastic "github.com/olivere/elastic/v7"
)

// compressPayload compresses the payload of an AWS request before it is sent.
// src: https://github.com/cloudposse/prometheus-to-cloudwatch/blob/master/prometheus_to_cloudwatch.go#L237
func compressPayload(r *aws.Request) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.Copy(zw, r.GetBody()); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	r.SetBufferBody(buf.Bytes())
	r.HTTPRequest.Header.Set("Content-Encoding", "gzip")
}

// statsRespNodes extracts a slice of nodes from a node stats API
// response, sorted by node name.
func statsRespNodes(r *elastic.NodesStatsResponse) []*elastic.NodesStatsNode {
	out := make([]*elastic.NodesStatsNode, 0, len(r.Nodes))
	for _, n := range r.Nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
