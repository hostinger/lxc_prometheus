package main

import (
    "flag"
    "net/http"
    "sync"
    "os/exec"
    "strconv"
    "strings"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/common/log"
)

const (
    namespace = "lxc"
)

var (
    listeningAddress    = flag.String("telemetry.address", ":9119", "Address on which to expose metrics")
    metricsEndpoint     = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics")
    containerLabels     = []string{"id"}
    metrics             = map[string]*prometheus.Desc {
        "cpu":          newDesc("cpu", containerLabels),
        "memory":       newDesc("memory", containerLabels),
        "total_bytes":  newDesc("total_bytes", containerLabels),
        "rx_bytes":     newDesc("rx_bytes", containerLabels),
        "tx_bytes":     newDesc("tx_bytes", containerLabels),
        "io":           newDesc("io", containerLabels),
    }
    metricsMapping      = map[string]string {
        "cpu":          "CPU use",
        "memory":       "Memory usage",
        "total_bytes":  "Total bytes",
        "rx_bytes":     "RX bytes",
        "tx_bytes":     "TX bytes",
        "io":           "BlkIO use",
    }
)

type Exporter struct {
    mutex sync.Mutex
    up *prometheus.Desc
    metricsCounter map[string]*prometheus.Desc
    scrapeFailures prometheus.Counter
}

func newDesc(metricName string, labels []string) *prometheus.Desc {
	return prometheus.NewDesc(
        prometheus.BuildFQName(namespace, "", metricName),
        metricsMapping[metricName],
        labels,
        nil)
}

func Containers() []int {
    var containers = []int{}
    output, err := exec.Command("lxc-ls", "--active", "-1").Output()

    if err != nil {
        log.Errorf("Error while listing LXC containers: ", err)
    }

    for _, id := range strings.Split(string(output), "\n") {
        if id != "" {
            i, err := strconv.Atoi(id)
            if err != nil {
                log.Errorf("Cannot parse LXC id: ", err)
            }
            containers = append(containers, i)
        }
    }
    return containers
}

func NewExporter() *Exporter {
    return &Exporter{
        up: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "up"),
            "Could the LXC exporter be reached",
            nil,
            nil),
        scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
            Namespace: namespace,
            Name:      "exporter_scrape_failures_total",
            Help:      "Number of errors while scraping LXC containers",
            }),
        metricsCounter: metrics,
    }
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
        ch <- e.up
        e.scrapeFailures.Describe(ch)
    for _, metric := range e.metricsCounter {
        ch <- metric
    }
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
    ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1)
    for _, id := range Containers() {
        container_id := strconv.Itoa(id)
        output, err := exec.Command("lxc-info", "-S", "-H", "-n", container_id).Output()

        if err != nil {
            log.Errorf("Error while getting stats for container (%d): ", container_id, err)
        }

        for _, info := range strings.Split(string(output), "\n") {
            if info != "" {
                info := strings.Split(info, ":")
                metric := strings.Trim(info[0], " ")
                valueString := strings.Trim(info[1], " ")
                valueInt, _ := strconv.ParseFloat(valueString, 64)
                for key, val := range metricsMapping {
                    if metric == val {
                        ch <- prometheus.MustNewConstMetric(e.metricsCounter[key], prometheus.CounterValue, valueInt, container_id)
                    }
                }
            }
        }
    }
    return nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
    e.mutex.Lock()
    defer e.mutex.Unlock()
    if err := e.collect(ch); err != nil {
        log.Errorf("Error scraping: %s", err)
        e.scrapeFailures.Inc()
        e.scrapeFailures.Collect(ch)
    }
    return
}

func main() {
    flag.Parse()

    exporter := NewExporter()
    prometheus.MustRegister(exporter)

    log.Infof("Starting Server: %s", *listeningAddress)
    http.Handle(*metricsEndpoint, prometheus.Handler())
    log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
