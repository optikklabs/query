package infraconsts

// OpenTelemetry Semantic Conventions for Infrastructure & Resource Metrics.
// Reference: https://opentelemetry.io/docs/specs/semconv/system/

const (
	TableMetrics = "optikk.metrics"

	ColTenantID    = "tenant_id"
	ColTimestamp   = "timestamp"
	ColMetricName  = "metric_name"
	ColValue       = "value"
	ColServiceName = "service"
	ColHost        = "host"
	ColPod         = "pod"
	ColContainer   = "container"
	ColAttributes  = "attributes"
	ColCount       = "hist_count"

	MetricSystemCPUUtilization = "system.cpu.utilization"
	MetricSystemCPUUsage       = "system.cpu.usage"
	MetricProcessCPUUsage      = "process.cpu.usage"
	MetricSystemCPUTime        = "system.cpu.time"
	MetricSystemCPULoadAvg1m   = "system.cpu.load_average.1m"
	MetricSystemCPULoadAvg5m   = "system.cpu.load_average.5m"
	MetricSystemCPULoadAvg15m  = "system.cpu.load_average.15m"
	MetricSystemProcessCount   = "system.process.count"

	MetricSystemMemoryUtilization = "system.memory.utilization"
	MetricSystemMemoryUsage       = "system.memory.usage"
	MetricSystemPagingUsage       = "system.paging.usage"

	MetricSystemDiskUtilization = "system.disk.utilization"
	MetricSystemDiskIO          = "system.disk.io"
	MetricSystemDiskOperations  = "system.disk.operations"
	MetricSystemDiskIOTime      = "system.disk.io_time"
	MetricSystemFilesystemUsage = "system.filesystem.usage"
	MetricSystemFilesystemUtil  = "system.filesystem.utilization"
	MetricDiskFree              = "disk.free"
	MetricDiskTotal             = "disk.total"

	MetricSystemNetworkUtilization = "system.network.utilization"
	MetricSystemNetworkIO          = "system.network.io"
	MetricSystemNetworkPackets     = "system.network.packets"
	MetricSystemNetworkErrors      = "system.network.errors"
	MetricSystemNetworkDropped     = "system.network.dropped"
	MetricSystemNetworkConnections = "system.network.connections"

	MetricJVMMemoryUsed        = "jvm.memory.used"
	MetricJVMMemoryMax         = "jvm.memory.max"
	MetricJVMMemoryCommitted   = "jvm.memory.committed"
	MetricJVMMemoryLimit       = "jvm.memory.limit"
	MetricJVMGCDuration        = "jvm.gc.duration"
	MetricJVMThreadCount       = "jvm.thread.count"
	MetricJVMClassLoaded       = "jvm.class.loaded"
	MetricJVMClassCount        = "jvm.class.count"
	MetricJVMCPUTime           = "jvm.cpu.time"
	MetricJVMCPUUtilization    = "jvm.cpu.recent_utilization"
	MetricJVMBufferMemoryUsage = "jvm.buffer.memory.usage"
	MetricJVMBufferCount       = "jvm.buffer.count"

	MetricDBConnectionPoolUtilization = "db.connection.pool.utilization"
	MetricHikariCPConnectionsActive   = "hikaricp.connections.active"
	MetricHikariCPConnectionsMax      = "hikaricp.connections.max"
	MetricJDBCConnectionsActive       = "jdbc.connections.active"
	MetricJDBCConnectionsMax          = "jdbc.connections.max"

	AttrSystemCPUState              = "system.cpu.state"
	AttrSystemCPUUtilization        = "system.cpu.utilization"
	AttrSystemMemoryState           = "system.memory.state"
	AttrSystemMemoryUtilization     = "system.memory.utilization"
	AttrSystemDiskDirection         = "system.disk.direction"
	AttrSystemDiskUtilization       = "system.disk.utilization"
	AttrSystemNetworkDirection      = "system.network.io.direction"
	AttrSystemNetworkState          = "system.network.state"
	AttrSystemNetworkUtilization    = "system.network.utilization"
	AttrFilesystemMountpoint        = "system.filesystem.mountpoint"
	AttrProcessStatus               = "process.status"
	AttrJVMMemoryType               = "jvm.memory.type"
	AttrJVMMemoryPoolName           = "jvm.memory.pool.name"
	AttrJVMGCName                   = "jvm.gc.name"
	AttrJVMGCAction                 = "jvm.gc.action"
	AttrJVMThreadDaemon             = "jvm.thread.daemon"
	AttrJVMBufferPoolName           = "jvm.buffer.pool.name"
	AttrDBConnectionPoolUtilization = "db.connection_pool.utilization"

	PercentageMultiplier = 100.0
	PercentageThreshold  = 1.0
)

var (
	CPUMetrics = []string{
		MetricSystemCPUUtilization,
		MetricSystemCPUUsage,
		MetricProcessCPUUsage,
	}

	MemoryMetrics = []string{
		MetricSystemMemoryUtilization,
		MetricJVMMemoryUsed,
		MetricJVMMemoryMax,
	}

	DiskMetrics = []string{
		MetricSystemDiskUtilization,
		MetricDiskFree,
		MetricDiskTotal,
	}

	NetworkMetrics = []string{
		MetricSystemNetworkUtilization,
	}

	ConnectionPoolMetrics = []string{
		MetricDBConnectionPoolUtilization,
		MetricHikariCPConnectionsActive,
		MetricHikariCPConnectionsMax,
		MetricJDBCConnectionsActive,
		MetricJDBCConnectionsMax,
	}

	AllResourceMetrics = []string{
		MetricSystemCPUUtilization,
		MetricSystemCPUUsage,
		MetricProcessCPUUsage,
		MetricSystemMemoryUtilization,
		MetricJVMMemoryUsed,
		MetricJVMMemoryMax,
		MetricSystemDiskUtilization,
		MetricDiskFree,
		MetricDiskTotal,
		MetricSystemNetworkUtilization,
		MetricDBConnectionPoolUtilization,
		MetricHikariCPConnectionsActive,
		MetricHikariCPConnectionsMax,
		MetricJDBCConnectionsActive,
		MetricJDBCConnectionsMax,
	}
)
