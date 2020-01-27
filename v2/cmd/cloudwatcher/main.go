package main

import (
	"os"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/v2/internal/app/cloudwatcher" // App implementation.
)

func main() {
	app, err := cloudwatcher.NewApp(prometheus.DefaultRegisterer)
	if err != nil {
		panic(err)
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
	app.Main(prometheus.DefaultGatherer)
}
