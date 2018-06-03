package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net"
	_ "net/http/pprof"
	"strconv"
	"strings"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace       = "solr"
	pidFileHelpText = `Path to Solr pid file.

        If provided, the standard process metrics get exported for the Solr
        process, prefixed with 'solr_process_...'. The solr_process exporter
        needs to have read access to files owned by the Solr process. Depends on
        the availability of /proc.

        https://prometheus.io/docs/instrumenting/writing_clientlibs/#process-metrics.`
)

var (
	listenAddress    = kingpin.Flag("unix-sock", "Address to listen on for unix sock access.").Default("/dev/shm/solr_detail_exporter.sock").String()
	metricsPath      = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	solrURI          = kingpin.Flag("solr.address", "URI on which to scrape Solr.").Default("http://localhost:8080").String()
	solrContextPath  = kingpin.Flag("solr.context-path", "Solr webapp context path.").Default("/solr").String()
	solrExcludedCore = kingpin.Flag("solr.excluded-core", "Regex to exclude core from monitoring").Default("").String()
	solrTimeout      = kingpin.Flag("solr.timeout", "Timeout for trying to get stats from Solr.").Default("5s").Duration()
	solrPidFile      = kingpin.Flag("solr.pid-file", "").Default(pidFileHelpText).String()
)

var landingPage = []byte(`<html>
<head><title>Solr detail exporter</title></head>
<body>
<h1>Solr detail exporter</h1>
<p><a href='` + *metricsPath + `'>Metrics</a></p>
</body>
</html>
`)

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("solr_detail_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	exporter := NewExporter(*solrURI, *solrContextPath, *solrTimeout, *solrExcludedCore)
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("solr_detail_exporter"))

	if *solrPidFile != "" {
		procExporter := prometheus.NewProcessCollectorPIDFn(
			func() (int, error) {
				content, err := ioutil.ReadFile(*solrPidFile)
				if err != nil {
					return 0, fmt.Errorf("Can't read pid file: %s", err)
				}
				value, err := strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					return 0, fmt.Errorf("Can't parse pid file: %s", err)
				}
				return value, nil
			}, "solr")
		prometheus.MustRegister(procExporter)
	}

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, prometheus.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(landingPage)
	})
	server := http.Server{
		Handler: mux, // http.DefaultServeMux,
	}
	os.Remove(*listenAddress)

	listener, err := net.Listen("unix", *listenAddress)
	if err != nil {
		panic(err)
	}
	server.Serve(listener)
}