package main

import (
	"os"

	"github.com/prometheus/client_golang/prometheus" // Prometheus metrics.
	kingpin "gopkg.in/alecthomas/kingpin.v2"         // Command line flag parsing.

	"github.com/mintel/elasticsearch-asg/internal/app/drainer" // App implementation.
)

func main() {
	app, err := drainer.NewApp(prometheus.DefaultRegisterer)
	if err != nil {
		panic(err)
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
	app.Main(prometheus.DefaultGatherer)
}
