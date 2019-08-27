package main

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/mintel/elasticsearch-asg/internal/app/cloudwatcher"
)

func main() {
	app, err := cloudwatcher.NewApp(prometheus.DefaultRegisterer)
	if err != nil {
		panic(err)
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
	app.Main(prometheus.DefaultGatherer)
}
