package cloudwatcher

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/dgraph-io/ristretto"                 // Cache.
	elastic "github.com/olivere/elastic/v7"          // Elasticsearch client.
	"github.com/pkg/errors"                          // Wrap errors with stacktrace.
	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	"go.uber.org/zap"                                // Logging.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/cloudwatchiface"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/ec2iface"

	"github.com/mintel/elasticsearch-asg/internal/pkg/cmd"     // Common command line app tools.
	"github.com/mintel/elasticsearch-asg/internal/pkg/metrics" // Prometheus metrics tools.
	"github.com/mintel/elasticsearch-asg/pkg/es"               // Extensions to the Elasticsearch client.
)

const (
	Name  = "cloudwatcher"
	Usage = "Push Elasticsearch metrics to AWS CloudWatch, specifically to run AWS Autoscaling Groups for Elasticsearch."

	// Batch size when pushing metrics to CloudWatch.
	// This is the max allowed by the AWS API.
	_batchSize = 20
)

// App holds application state.
type App struct {
	*kingpin.Application

	flags  *Flags           // Command line flags
	health *Healthchecks    // healthchecks HTTP handler
	inst   *Instrumentation // App-specific Prometheus metrics

	// API clients.
	clients struct {
		ElasticsearchHTTP *http.Client
		Elasticsearch     *elastic.Client

		CloudWatch cloudwatchiface.ClientAPI
		EC2        ec2iface.ClientAPI
	}

	// A cache for storeing vCPU counts for each
	// Elasticsearch node.
	ec2Instances *ristretto.Cache
}

// NewApp returns a new App.
func NewApp(r prometheus.Registerer) (*App, error) {
	namespace := cmd.BuildPromFQName("", Name)

	app := &App{
		Application: kingpin.New(filepath.Base(os.Args[0]), Usage),
		health:      NewHealthchecks(r, namespace),
	}
	// create a cache instance
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000 * 10,
		MaxCost:     1000,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		return nil, err
	}
	app.ec2Instances = cache
	app.flags = NewFlags(app.Application)
	app.inst = NewInstrumentation(namespace, app.ec2Instances)
	if err := r.Register(app.inst); err != nil {
		return nil, err
	}

	// Add post-flag-parsing actions.
	// These should only return an error if that error
	// is related to user input in some way, since kingpin prints the
	// error in a way that suggests a user problem. For example, an error
	// connecting to Elasticsearch might look like:
	//
	//   cloudwatcher: error: health check timeout: no Elasticsearch node available, try --help

	// Instrument a HTTP client that will be used to connect
	// to Elasticsearch. Don't create the Elasticsearch client
	// itself since the client makes an immeditate call to
	// Elasticsearch to check the connection.
	app.Action(func(*kingpin.ParseContext) error {
		constLabels := map[string]string{"recipient": "elasticsearch"}
		c, err := metrics.InstrumentHTTP(nil, r, namespace, constLabels)
		if err != nil {
			panic("error instrumenting HTTP client: " + err.Error())
		}
		app.clients.ElasticsearchHTTP = c
		return nil
	})

	// Set up AWS client(s).
	app.Action(func(*kingpin.ParseContext) error {
		cfg := app.flags.AWSConfig()
		err := metrics.InstrumentAWS(&cfg.Handlers, r, namespace, nil)
		if err != nil {
			panic("error instrumenting AWS config: " + err.Error())
		}
		app.clients.CloudWatch = cloudwatch.New(cfg)
		app.clients.EC2 = ec2.New(cfg)
		app.health.AWSSessionCreated = true
		return nil
	})

	return app, nil
}

// Main is the main method of App and should be called
// in main.main() after flag parsing.
func (app *App) Main(g prometheus.Gatherer) {
	logger := app.flags.NewLogger()
	defer func() { _ = logger.Sync() }()
	defer cmd.SetGlobalLogger(logger)()

	// Set up Elasticsearch client.
	c, err := app.flags.NewElasticsearchClient(
		elastic.SetHttpClient(app.clients.ElasticsearchHTTP),
	)
	if err != nil {
		logger.Fatal("error connecting to Elasticsearch", zap.Error(err))
	}
	defer c.Stop()
	app.clients.Elasticsearch = c
	app.health.ElasticSessionCreated = true

	// Serve the healthchecks, Prometheus metrics, and pprof traces.
	go func() {
		mux := app.flags.ConfigureMux(http.DefaultServeMux, app.health.Handler, g)
		srv := app.flags.NewServer(mux)
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatal("error serving healthchecks/metrics", zap.Error(err))
		}
	}()

	clusterName, err := app.getClusterName()
	if err != nil {
		logger.Fatal("error while getting cluster name", zap.Error(err))
	}

	ticker := app.flags.Tick()
	for range ticker {
		stats, err := app.getStats()
		if err != nil {
			logger.Fatal("error while getting node stats", zap.Error(err))
		}

		roles := make(map[string]NodeStatsSlice)
		for _, n := range stats {
			roles["all"] = append(roles["all"], n)
			if len(n.Roles) == 0 {
				roles["coordinate"] = append(roles["coordinate"], n)
			} else {
				for _, r := range n.Roles {
					roles[r] = append(roles[r], n)
				}
			}
		}

		var metricData []cloudwatch.MetricDatum
		for r, s := range roles {
			dimensions := []cloudwatch.Dimension{
				cloudwatch.Dimension{
					Name:  aws.String("ClusterName"),
					Value: aws.String(clusterName),
				},
				cloudwatch.Dimension{
					Name:  aws.String("Role"),
					Value: aws.String(r),
				},
			}
			data := s.Aggregate(dimensions)
			metricData = append(metricData, data...)
		}

		logger.Info("pushing metrics to CloudWatch", zap.Int("count", len(metricData)))
		if err = app.pushCloudwatchData(metricData); err != nil {
			logger.Fatal("error pushing metrics to CloudWatch", zap.Error(err))
		}

		app.inst.Loops.Inc()
	}
}

// getClusterName returns the name of the Elasticsearch cluster.
func (app *App) getClusterName() (string, error) {
	zap.L().Debug("getting cluster name")
	r, err := app.clients.Elasticsearch.ClusterHealth().Do(context.Background())
	if err != nil {
		return "", err
	}
	return r.ClusterName, nil
}

// getStats returns a slice of NodeStats
func (app *App) getStats() (NodeStatsSlice, error) {
	nodes, err := app.getNodes()
	if err != nil {
		err = errors.Wrap(err, "error getting Elasticsearch nodes info")
		return nil, err
	}

	instanceIDs := make([]string, len(nodes))
	for i, n := range nodes {
		instanceIDs[i] = n.Name
	}

	instances, err := app.getInstances(instanceIDs)
	if err != nil {
		return nil, errors.Wrap(err, "error describing EC2 instances")
	}
	if len(instanceIDs) != len(nodes) {
		return nil, errors.Wrap(
			errInconsistentNodes,
			"got different number of Elasticsearch nodes and EC2 instances",
		)
	}

	settings, err := app.getSettings()
	if err != nil {
		return nil, errors.Wrap(err, "error getting Elasticsearch settings")
	}
	transient := es.NewShardAllocationExcludeSettings(settings.Transient)
	persistent := es.NewShardAllocationExcludeSettings(settings.Persistent)

	stats := make(NodeStatsSlice, len(nodes))
	for i := range nodes {
		s, err := NewNodeStats(
			nodes[i],
			instances[i],
			transient,
			persistent,
		)
		if err != nil {
			return nil, err
		}
		stats[i] = s
	}
	return stats, nil
}

// getNodes
func (app *App) getNodes() ([]*elastic.NodesStatsNode, error) {
	zap.L().Debug("getting node stats")
	resp, err := app.clients.Elasticsearch.NodesStats().Metric("os", "jvm", "fs").Do(context.Background())
	if err != nil {
		return nil, err
	}
	return statsRespNodes(resp), nil
}

// getInstances gets EC2 instance information from the AWS EC2 API.
// The returned result is sorted based on instance ID.
func (app *App) getInstances(IDs []string) ([]*EC2Instance, error) {
	instances := make([]*EC2Instance, 0, len(IDs))
	toDescribe := make([]string, 0, len(IDs))
	for _, i := range IDs {
		if inst, ok := app.ec2Instances.Get(i); ok {
			instances = append(instances, inst.(*EC2Instance))
		} else {
			toDescribe = append(toDescribe, i)
		}
	}
	if len(toDescribe) != 0 {
		zap.L().Debug("describing instances", zap.Strings("instance_ids", toDescribe))
		req := app.clients.EC2.DescribeInstancesRequest(&ec2.DescribeInstancesInput{
			InstanceIds: toDescribe,
		})
		p := ec2.NewDescribeInstancesPaginator(req)
		for p.Next(context.Background()) {
			page := p.CurrentPage()
			for _, r := range page.Reservations {
				for _, i := range r.Instances {
					inst := NewEC2Instance(i)
					instances = append(instances, inst)
					app.ec2Instances.Set(inst.ID, inst, 1)
				}
			}
		}
		if err := p.Err(); err != nil {
			return nil, err
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})
	return instances, nil
}

// getSettings gets the cluster settings from Elasticsearch.
func (app *App) getSettings() (*es.ClusterGetSettingsResponse, error) {
	zap.L().Debug("getting cluster settings")
	s := es.NewClusterGetSettingsService(app.clients.Elasticsearch)
	s = s.FilterPath("*." + es.ShardAllocExcludeSetting + ".*")
	return s.Do(context.Background())
}

// pushCloudwatchData pushes metrics to CloudWatch.
//
// The CloudWatch API has the following limitations:
//  - Max 40kb request size
//	- Single namespace per request
//	- Max 10 dimensions per metric
// Send metrics compressed and in batches.
func (app *App) pushCloudwatchData(data []cloudwatch.MetricDatum) error {
	for i := 0; i < len(data); i += _batchSize {
		j := i + _batchSize
		if j > len(data) {
			j = len(data)
		}
		batch := data[i:j]
		req := app.clients.CloudWatch.PutMetricDataRequest(&cloudwatch.PutMetricDataInput{
			Namespace:  aws.String(app.flags.Namespace),
			MetricData: batch,
		})
		req.Handlers.Build.PushBack(compressPayload)
		zap.L().Debug("pushing batch of metrics to CloudWatch", zap.Int("count", len(batch)))
		if _, err := req.Send(context.Background()); err != nil {
			return err
		}
		app.inst.MetricsPushed.Add(float64(len(batch)))
	}
	return nil
}
