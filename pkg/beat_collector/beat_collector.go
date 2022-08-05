package beat_collector

import (
	"encoding/json"
	"github.com/elastic/beats/v7/libbeat/monitoring"
	"github.com/trustpilot/beat-exporter/collector"
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type BeatCollector struct {
	Collectors map[string]prometheus.Collector
	Stats      *collector.Stats
	name       string
	beatInfo   *collector.BeatInfo
	targetDesc *prometheus.Desc
	targetUp   *prometheus.Desc
	metrics    exportedMetrics
}

type exportedMetrics []struct {
	desc    *prometheus.Desc
	eval    func(stats *collector.Stats) float64
	valType prometheus.ValueType
}

// HackfixRegex regex to replace JSON part
var HackfixRegex = regexp.MustCompile("\"time\":(\\d+)") // replaces time:123 to time.ms:123, only filebeat has different naming of time metric

// NewMainCollector constructor
func NewMainCollector(name string, beatInfo *collector.BeatInfo) prometheus.Collector {
	beat := &BeatCollector{
		Collectors: make(map[string]prometheus.Collector),
		Stats:      &collector.Stats{},
		name:       name,
		targetDesc: prometheus.NewDesc(
			prometheus.BuildFQName(name, "target", "info"),
			"target information",
			nil,
			prometheus.Labels{"version": beatInfo.Version, "beat": beatInfo.Beat, "uri": ""}),
		targetUp: prometheus.NewDesc(
			prometheus.BuildFQName("", beatInfo.Beat, "up"),
			"Target up",
			nil,
			nil),

		beatInfo: beatInfo,
		metrics:  exportedMetrics{},
	}

	beat.Collectors["system"] = collector.NewSystemCollector(beatInfo, beat.Stats)
	beat.Collectors["beat"] = collector.NewBeatCollector(beatInfo, beat.Stats)
	beat.Collectors["libbeat"] = collector.NewLibBeatCollector(beatInfo, beat.Stats)
	beat.Collectors["registrar"] = collector.NewRegistrarCollector(beatInfo, beat.Stats)
	beat.Collectors["filebeat"] = collector.NewFilebeatCollector(beatInfo, beat.Stats)
	beat.Collectors["metricbeat"] = collector.NewMetricbeatCollector(beatInfo, beat.Stats)

	return beat
}

// Describe returns all descriptions of the collector.
func (b *BeatCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- b.targetDesc
	ch <- b.targetUp

	for _, metric := range b.metrics {
		ch <- metric.desc
	}

	// standard collectors for all types of beats
	b.Collectors["system"].Describe(ch)
	b.Collectors["beat"].Describe(ch)
	b.Collectors["libbeat"].Describe(ch)

	// Customized collectors per beat type
	switch b.beatInfo.Beat {
	case "filebeat":
		b.Collectors["filebeat"].Describe(ch)
		b.Collectors["registrar"].Describe(ch)
	case "metricbeat":
		b.Collectors["metricbeat"].Describe(ch)
	}

}

// Collect returns the current state of all metrics of the collector.
func (b *BeatCollector) Collect(ch chan<- prometheus.Metric) {

	err := b.fetchStatsEndpoint()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(b.targetUp, prometheus.GaugeValue, float64(0)) // set target down
		log.Errorf("Failed getting /stats endpoint of target: " + err.Error())
		return
	}

	ch <- prometheus.MustNewConstMetric(b.targetDesc, prometheus.GaugeValue, float64(1))
	ch <- prometheus.MustNewConstMetric(b.targetUp, prometheus.GaugeValue, float64(1)) // target up

	for _, i := range b.metrics {
		ch <- prometheus.MustNewConstMetric(i.desc, i.valType, i.eval(b.Stats))
	}

	// standard collectors for all types of beats
	b.Collectors["system"].Collect(ch)
	b.Collectors["beat"].Collect(ch)
	b.Collectors["libbeat"].Collect(ch)

	// Customized collectors per beat type
	switch b.beatInfo.Beat {
	case "filebeat":
		b.Collectors["filebeat"].Collect(ch)
		b.Collectors["registrar"].Collect(ch)
	case "metricbeat":
		b.Collectors["metricbeat"].Collect(ch)
	}

}

func (b *BeatCollector) fetchStatsEndpoint() error {

	/*	response, err := b.client.Get(b.beatURL.String() + "/stats")
		if err != nil {
			log.Errorf("Could not fetch stats endpoint of target: %v", b.beatURL.String())
			return err
		}

		defer response.Body.Close()

		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Error("Can't read body of response")
			return err
		}
	*/
	data := monitoring.CollectStructSnapshot(
		monitoring.GetNamespace("stats").GetRegistry(),
		monitoring.Full,
		false,
	)
	bodyBytes, err := json.Marshal(data)
	if err != nil {
		log.Error("Can't read CollectStructSnapshot")
		return err
	}

	// @TODO remove this when filebeat stats endpoint output matches all other beats output
	bodyBytes = HackfixRegex.ReplaceAll(bodyBytes, []byte("\"time\":{\"ms\":$1}"))

	err = json.Unmarshal(bodyBytes, &b.Stats)
	if err != nil {
		log.Error("Could not parse JSON response for target")
		return err
	}

	return nil
}
