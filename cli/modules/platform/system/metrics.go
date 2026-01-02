package system

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// Metrics holds system resource usage information
type Metrics struct {
	CPUPercent float64   // CPU usage percentage (0-100)
	MemUsedGB  float64   // Memory used in GB
	MemTotalGB float64   // Total memory in GB
	MemPercent float64   // Memory usage percentage (0-100)
	LoadAvg1   float64   // 1 minute load average
	LoadAvg5   float64   // 5 minute load average
	LoadAvg15  float64   // 15 minute load average
	NumCPU     int       // Number of CPUs
	UpdatedAt  time.Time // When metrics were last updated
}

// MetricsCollector collects system metrics
type MetricsCollector struct {
	mu          sync.RWMutex
	metrics     Metrics
	refreshRate time.Duration
	stopCh      chan struct{}
	running     bool
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(refreshRate time.Duration) *MetricsCollector {
	if refreshRate < time.Second {
		refreshRate = time.Second
	}
	return &MetricsCollector{
		refreshRate: refreshRate,
		stopCh:      make(chan struct{}),
		metrics:     Metrics{NumCPU: runtime.NumCPU()},
	}
}

// Start begins collecting metrics periodically
func (mc *MetricsCollector) Start() {
	mc.mu.Lock()
	if mc.running {
		mc.mu.Unlock()
		return
	}
	mc.running = true
	mc.mu.Unlock()

	// Initial collection
	mc.collect()

	go func() {
		ticker := time.NewTicker(mc.refreshRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				mc.collect()
			case <-mc.stopCh:
				return
			}
		}
	}()
}

// Stop stops the metrics collection
func (mc *MetricsCollector) Stop() {
	mc.mu.Lock()
	if mc.running {
		mc.running = false
		close(mc.stopCh)
	}
	mc.mu.Unlock()
}

// Get returns the current metrics
func (mc *MetricsCollector) Get() Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.metrics
}

// collect gathers all metrics
func (mc *MetricsCollector) collect() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.collectCPU()
	mc.collectMemory()
	mc.collectLoadAvg()
	mc.metrics.NumCPU = runtime.NumCPU()
	mc.metrics.UpdatedAt = time.Now()
}

// collectCPU reads CPU usage using gopsutil
func (mc *MetricsCollector) collectCPU() {
	// Get CPU percent over 500ms interval (non-blocking with previous sample)
	percents, err := cpu.Percent(0, false)
	if err != nil || len(percents) == 0 {
		return
	}
	mc.metrics.CPUPercent = percents[0]
}

// collectMemory reads memory info using gopsutil
func (mc *MetricsCollector) collectMemory() {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	// Convert bytes to GB
	mc.metrics.MemTotalGB = float64(vmStat.Total) / 1024 / 1024 / 1024
	mc.metrics.MemUsedGB = float64(vmStat.Used) / 1024 / 1024 / 1024
	mc.metrics.MemPercent = vmStat.UsedPercent
}

// collectLoadAvg reads load average using gopsutil
func (mc *MetricsCollector) collectLoadAvg() {
	avgStat, err := load.Avg()
	if err != nil {
		// Load average not available on Windows, use 0
		mc.metrics.LoadAvg1 = 0
		mc.metrics.LoadAvg5 = 0
		mc.metrics.LoadAvg15 = 0
		return
	}

	mc.metrics.LoadAvg1 = avgStat.Load1
	mc.metrics.LoadAvg5 = avgStat.Load5
	mc.metrics.LoadAvg15 = avgStat.Load15
}
