package metrics

import (
	"log"

	lxd "github.com/lxc/lxd/client"
	lxdapi "github.com/lxc/lxd/shared/api"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector collects metrics to be sent to Prometheus.
type collector struct {
	logger *log.Logger
	server lxd.InstanceServer
}

// NewCollector creates a new collector with logger and LXD connection.
func NewCollector(
	logger *log.Logger, server lxd.InstanceServer) prometheus.Collector {
	return &collector{logger: logger, server: server}
}

var (
	cpuUsageDesc = prometheus.NewDesc("lxd_container_cpu_usage",
		"Container CPU Usage in Seconds",
		[]string{"container_name", "location"}, nil,
	)
	diskUsageDesc = prometheus.NewDesc("lxd_container_disk_usage",
		"Container Disk Usage",
		[]string{"container_name", "location", "disk_device"}, nil,
	)
	networkUsageDesc = prometheus.NewDesc("lxd_container_network_usage",
		"Container Network Usage",
		[]string{"container_name", "location", "interface", "operation"}, nil,
	)

	memUsageDesc = prometheus.NewDesc("lxd_container_mem_usage",
		"Container Memory Usage",
		[]string{"container_name", "location"}, nil,
	)
	memUsagePeakDesc = prometheus.NewDesc("lxd_container_mem_usage_peak",
		"Container Memory Usage Peak",
		[]string{"container_name", "location"}, nil,
	)
	swapUsageDesc = prometheus.NewDesc("lxd_container_swap_usage",
		"Container Swap Usage",
		[]string{"container_name", "location"}, nil,
	)
	swapUsagePeakDesc = prometheus.NewDesc("lxd_container_swap_usage_peak",
		"Container Swap Usage Peak",
		[]string{"container_name", "location"}, nil,
	)

	processCountDesc = prometheus.NewDesc("lxd_container_process_count",
		"Container number of process Running",
		[]string{"container_name", "location"}, nil,
	)
	containerPIDDesc = prometheus.NewDesc("lxd_container_pid",
		"Container PID",
		[]string{"container_name", "location"}, nil,
	)
	runningStatusDesc = prometheus.NewDesc("lxd_container_running_status",
		"Container Running Status",
		[]string{"container_name", "location"}, nil,
	)
)

// Describe fills given channel with metrics descriptor.
func (collector *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- cpuUsageDesc
	ch <- memUsageDesc
	ch <- memUsagePeakDesc
	ch <- swapUsageDesc
	ch <- swapUsagePeakDesc
	ch <- processCountDesc
	ch <- containerPIDDesc
	ch <- runningStatusDesc
	ch <- diskUsageDesc
	ch <- networkUsageDesc
}

// Collect fills given channel with metrics data.
func (collector *collector) Collect(ch chan<- prometheus.Metric) {
	containerNames, err := collector.server.GetContainerNames()
	if err != nil {
		collector.logger.Printf("Can't query container names: %s", err)
		return
	}

	for _, containerName := range containerNames {
		state, _, err := collector.server.GetContainerState(containerName)
		containerInfo, _, err2 := collector.server.GetContainer(containerName)
		if err != nil {
			collector.logger.Printf(
				"Can't query container state for `%s`: %s", containerName, err)
			continue
		}

		if err2 != nil {
			collector.logger.Printf(
				"Can't query container location for `%s`: %s", containerName, err2)
			continue
		}

		collector.collectContainerMetrics(ch, containerName, containerInfo.Location, state)
	}
}

func (collector *collector) collectContainerMetrics(
	ch chan<- prometheus.Metric,
	containerName string,
	containerLocation string,
	state *lxdapi.ContainerState,
) {
	ch <- prometheus.MustNewConstMetric(cpuUsageDesc,
		prometheus.GaugeValue, float64(state.CPU.Usage), containerName, containerLocation)
	ch <- prometheus.MustNewConstMetric(processCountDesc,
		prometheus.GaugeValue, float64(state.Processes), containerName, containerLocation)
	ch <- prometheus.MustNewConstMetric(
		containerPIDDesc, prometheus.GaugeValue, float64(state.Pid), containerName, containerLocation)

	ch <- prometheus.MustNewConstMetric(memUsageDesc,
		prometheus.GaugeValue, float64(state.Memory.Usage), containerName, containerLocation)
	ch <- prometheus.MustNewConstMetric(memUsagePeakDesc,
		prometheus.GaugeValue, float64(state.Memory.UsagePeak), containerName, containerLocation)
	ch <- prometheus.MustNewConstMetric(swapUsageDesc,
		prometheus.GaugeValue, float64(state.Memory.SwapUsage), containerName, containerLocation)
	ch <- prometheus.MustNewConstMetric(swapUsagePeakDesc,
		prometheus.GaugeValue, float64(state.Memory.SwapUsagePeak), containerName, containerLocation)

	runningStatus := 0
	if state.Status == "Running" {
		runningStatus = 1
	}
	ch <- prometheus.MustNewConstMetric(runningStatusDesc,
		prometheus.GaugeValue, float64(runningStatus), containerName, containerLocation)

	for diskName, state := range state.Disk {
		ch <- prometheus.MustNewConstMetric(diskUsageDesc,
			prometheus.GaugeValue, float64(state.Usage), containerName, containerLocation, diskName)
	}

	for ethName, state := range state.Network {
		networkMetrics := map[string]int64{
			"BytesReceived":   state.Counters.BytesReceived,
			"BytesSent":       state.Counters.BytesSent,
			"PacketsReceived": state.Counters.PacketsReceived,
			"PacketsSent":     state.Counters.PacketsSent,
		}

		for opName, value := range networkMetrics {
			ch <- prometheus.MustNewConstMetric(networkUsageDesc,
				prometheus.GaugeValue, float64(value), containerName, containerLocation, ethName, opName)
		}
	}
}
