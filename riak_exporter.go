package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	listenAddress = flag.String("web.listen-address", ":9203", "Address to listen on for web interface and telemetry.")
	riakURI       = flag.String("riak.uri", "http://localhost:8098", "The URI which the Riak HTTP API listens on.")
	namespace     = "riak"
)

type Exporter struct {
	uri          string
	duration     prometheus.Gauge
	totalScrapes prometheus.Counter
	riakUp       prometheus.Gauge
	scrapeError  prometheus.Gauge
}

func NewExporter(uri string) *Exporter {
	return &Exporter{
		uri: uri,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Riak.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "scrapes_total",
			Help:      "Total number of times Riak was scraped for metrics.",
		}),
		riakUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Riak node is up.",
		}),
		scrapeError: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Riak resulted in an error (1 for error, 0 for success).",
		}),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(e, ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.scrape(ch)
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.riakUp
	ch <- e.scrapeError
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	startTime := time.Now()
	defer func() {
		e.duration.Set(time.Since(startTime).Seconds())
	}()

	pingResponse, err := http.Get(e.uri + "/ping")
	if err != nil {
		e.scrapeError.Set(1)
		logrus.Error("Error trying to ping the Riak node: ", err)
		return
	}
	defer pingResponse.Body.Close()

	if pingResponse.StatusCode != http.StatusOK {
		e.scrapeError.Set(1)
		logrus.Error("Riak node is down")
		return
	}
	e.riakUp.Set(1)
	e.scrapeError.Set(0)

	statsResponse, err := http.Get(e.uri + "/stats")
	if err != nil {
		logrus.Errorln("Error trying to fetch the stats for the Riak node")
		return
	}

	defer statsResponse.Body.Close()

	if statsResponse.StatusCode != http.StatusOK {
		e.scrapeError.Set(1)
		logrus.Errorln("Error when fetching the stats for the Riak node")
		return
	}

	data, err := io.ReadAll(statsResponse.Body)
	if err != nil {
		e.scrapeError.Set(1)
		logrus.Errorln("Error reading the response body for the /stats endpoint")
		return
	}

	var f map[string]interface{}
	if err = json.Unmarshal(data, &f); err != nil {
		e.scrapeError.Set(1)
		logrus.Errorln("Error parsing the Riak metrics")
		return
	}

	stringMetrics := map[string]string{
		"storage_backend":       "Configuration storage backend of Riak",
		"sys_driver_version":    "Driver version of Riak system",
		"sys_global_heaps_size": "Global heaps size of Riak system",
		"sys_heap_type":         "Heap type of Riak system",
		"sys_otp_release":       "OTP release of Riak system",
		"sys_system_version":    "System version of Riak",
	}

	for metricName, metricDesc := range stringMetrics {
		if value, ok := f[metricName].(string); ok {
			desc := prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "", metricName),
				metricDesc,
				[]string{"value"},
				nil,
			)
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, value)
			logrus.Infof("Creating metric %s with value %s", metricName, value)
		}
	}

	processNodeRelatedMetrics(f, ch)

	for metricName, metricValue := range f {
		if value, ok := metricValue.(float64); ok {
			desc := prometheus.NewDesc(
				prometheus.BuildFQName(namespace, "", metricName),
				metricName,
				nil,
				nil,
			)
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
		}
	}
}

func processNodeRelatedMetrics(f map[string]interface{}, ch chan<- prometheus.Metric) {
	nodeMetrics := []string{"connected_nodes", "ring_members"}
	for _, metricName := range nodeMetrics {
		if nodes, ok := f[metricName].([]interface{}); ok {
			totalDesc := prometheus.NewDesc(prometheus.BuildFQName(namespace, "", metricName+"_total"), "Total count of "+metricName, nil, nil)
			ch <- prometheus.MustNewConstMetric(totalDesc, prometheus.GaugeValue, float64(len(nodes)))

			for _, node := range nodes {
				if nodeName, ok := node.(string); ok {
					desc := prometheus.NewDesc(prometheus.BuildFQName(namespace, "", metricName), metricName, []string{"node"}, nil)
					ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, nodeName)
				}
			}
		}
	}
}

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	flag.Parse()

	registry := prometheus.NewRegistry()
	exporter := NewExporter(*riakURI)
	registry.MustRegister(exporter)

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	logrus.Infof("Listening on %s", *listenAddress)
	logrus.Fatal(http.ListenAndServe(*listenAddress, nil))
}
