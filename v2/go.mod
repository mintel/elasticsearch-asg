module github.com/mintel/elasticsearch-asg/v2

go 1.12

require (
	github.com/aws/aws-sdk-go-v2 v0.12.0
	github.com/dgraph-io/ristretto v0.0.1
	github.com/looplab/fsm v0.1.0
	github.com/mattn/go-isatty v0.0.9
	github.com/mintel/healthcheck v0.0.0-20190930173525-0ae502142f18
	github.com/olebedev/emitter v0.0.0-20190110104742-e8d1457e6aee
	github.com/olivere/elastic/v7 v7.0.17
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.1
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/tidwall/gjson v1.3.2
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.2.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/exp v0.0.0-20190927203820-447a159532ef // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	gonum.org/v1/gonum v0.0.0-20190929233944-b20cf7805fc4
	gonum.org/v1/netlib v0.0.0-20190926062253-2d6e29b73a19 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/h2non/gock.v1 v1.0.15
)

replace github.com/golang/lint => golang.org/x/lint v0.0.0-20190409202823-959b441ac422
