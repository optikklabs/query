package infraconsts

const (
	MetricSystemCPUUtilization = "system.cpu.utilization"
	MetricSystemCPUUsage       = "system.cpu.usage"
	MetricProcessCPUUsage      = "process.cpu.usage"
	MetricSystemCPULoadAvg1m   = "system.cpu.load_average.1m"
	MetricSystemCPULoadAvg5m   = "system.cpu.load_average.5m"
	MetricSystemCPULoadAvg15m  = "system.cpu.load_average.15m"
	MetricSystemProcessCount   = "system.process.count"

	MetricSystemMemoryUtilization = "system.memory.utilization"
	MetricSystemMemoryUsage       = "system.memory.usage"

	MetricSystemDiskUtilization = "system.disk.utilization"
	MetricSystemDiskIO          = "system.disk.io"
	MetricSystemFilesystemUtil  = "system.filesystem.utilization"
	MetricDiskFree              = "disk.free"
	MetricDiskTotal             = "disk.total"

	MetricSystemNetworkIO      = "system.network.io"
	MetricSystemNetworkErrors  = "system.network.errors"
	MetricSystemNetworkDropped = "system.network.dropped"

	MetricK8SPodCPUUtilization      = "k8s.pod.cpu.utilization"
	MetricK8SPodMemoryUsage         = "k8s.pod.memory.usage"
	MetricK8SPodMemoryWorkingSet    = "k8s.pod.memory.working_set"
	MetricK8SPodNetworkIO           = "k8s.pod.network.io"
	MetricK8SPodNetworkErrors       = "k8s.pod.network.errors"
	MetricK8SPodFilesystemUsage     = "k8s.pod.filesystem.usage"
	MetricK8SPodFilesystemCapacity  = "k8s.pod.filesystem.capacity"
	MetricK8SPodFilesystemAvailable = "k8s.pod.filesystem.available"
	MetricK8SContainerRestarts      = "k8s.container.restarts"
	MetricContainerCPUUtilization   = "container.cpu.utilization"
	MetricContainerMemoryUsage      = "container.memory.usage"

	MetricJVMMemoryUsed     = "jvm.memory.used"
	MetricJVMMemoryMax      = "jvm.memory.max"
	MetricJVMCPUUtilization = "jvm.cpu.recent_utilization"

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
)
