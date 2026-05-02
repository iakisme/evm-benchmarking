// Package report defines the on-disk output schema for bscbench runs.
package report

import "time"

const SchemaVersion = "1"

type Result struct {
	SchemaVersion string  `json:"schema_version"`
	Run           RunMeta `json:"run"`
	Sysinfo       Sysinfo `json:"sysinfo"`
	Metrics       Metrics `json:"metrics"`
	Passes        Passes  `json:"passes"`
}

type RunMeta struct {
	ID              string    `json:"id"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	BscbenchVersion string    `json:"bscbench_version"`
	BSCVersion      string    `json:"bsc_version"`
	InputHash       string    `json:"input_hash"`
	FromBlock       uint64    `json:"from_block"`
	ToBlock         uint64    `json:"to_block"`
	BlockCount      uint64    `json:"block_count"`
	WarmupState     string    `json:"warmup_state"`
	SkipWarmup      bool      `json:"skip_warmup"`
}

type Sysinfo struct {
	Host   HostInfo   `json:"host"`
	CPU    CPUInfo    `json:"cpu"`
	Memory MemInfo    `json:"memory"`
	Disk   []DiskInfo `json:"disk"`
	Go     GoInfo     `json:"go"`
	Cloud  *CloudInfo `json:"cloud"`
}

type HostInfo struct {
	Hostname  string `json:"hostname"`
	Kernel    string `json:"kernel"`
	OS        string `json:"os"`
	UptimeSec uint64 `json:"uptime_sec"`
}

type CPUInfo struct {
	Model         string   `json:"model"`
	CoresPhysical int      `json:"cores_physical"`
	CoresLogical  int      `json:"cores_logical"`
	FlagsSubset   []string `json:"flags_subset"`
	Governor      string   `json:"governor"`
	MhzBase       float64  `json:"mhz_base"`
}

type MemInfo struct {
	TotalBytes     uint64 `json:"total_bytes"`
	SwapBytes      uint64 `json:"swap_bytes"`
	HugepagesTotal uint64 `json:"hugepages_total"`
}

type DiskInfo struct {
	Device          string `json:"device"`
	Model           string `json:"model"`
	SizeBytes       uint64 `json:"size_bytes"`
	FS              string `json:"fs"`
	Mount           string `json:"mount"`
	Rotational      bool   `json:"rotational"`
	QueueScheduler  string `json:"queue_scheduler"`
	DiscardMaxBytes uint64 `json:"discard_max_bytes"`
}

type GoInfo struct {
	Version    string `json:"version"`
	GOMAXPROCS int    `json:"gomaxprocs"`
	GOGC       int    `json:"gogc"`
}

type CloudInfo struct {
	Provider     string `json:"provider"`
	InstanceType string `json:"instance_type"`
	AZ           string `json:"az"`
	Region       string `json:"region"`
}

type Metrics struct {
	Mgasps              float64 `json:"mgasps"`
	TotalGasUsed        uint64  `json:"total_gas_used"`
	TotalTxCount        uint64  `json:"total_tx_count"`
	RevertedTxCount     uint64  `json:"reverted_tx_count"`
	TotalWallSec        float64 `json:"total_wall_sec"`
	TxPerSec            float64 `json:"tx_per_sec"`
	BlockPerSec         float64 `json:"block_per_sec"`
	ExecNsP50           uint64  `json:"exec_ns_p50"`
	ExecNsP95           uint64  `json:"exec_ns_p95"`
	ExecNsP99           uint64  `json:"exec_ns_p99"`
	TrieCommitNsP50     uint64  `json:"trie_commit_ns_p50"`
	TrieCommitNsP95     uint64  `json:"trie_commit_ns_p95"`
	TrieCommitNsP99     uint64  `json:"trie_commit_ns_p99"`
	GasUsedPerBlockP50  uint64  `json:"gas_used_per_block_p50"`
	GasUsedPerBlockP95  uint64  `json:"gas_used_per_block_p95"`
	GasUsedPerBlockP99  uint64  `json:"gas_used_per_block_p99"`
	CPUPctAvg           float64 `json:"cpu_pct_avg"`
	CPUPctMax           float64 `json:"cpu_pct_max"`
	RSSPeakBytes        uint64  `json:"rss_peak_bytes"`
	DiskReadTotalBytes  uint64  `json:"disk_read_total_bytes"`
	DiskWriteTotalBytes uint64  `json:"disk_write_total_bytes"`
	DiskReadMBpsAvg     float64 `json:"disk_read_MBps_avg"`
	DiskWriteMBpsAvg    float64 `json:"disk_write_MBps_avg"`
}

type Passes struct {
	Warmup   PassMeta `json:"warmup"`
	Measured PassMeta `json:"measured"`
}

type PassMeta struct {
	WallSec float64 `json:"wall_sec"`
	GasUsed uint64  `json:"gas_used"`
}

// BlockRecord is one row of blocks.csv.
type BlockRecord struct {
	BlockNumber     uint64
	TxCount         uint32
	GasUsed         uint64
	ExecNs          uint64
	StateReadCount  uint64
	StateWriteCount uint64
	TrieCommitNs    uint64
	DBReadBytes     uint64
	DBWriteBytes    uint64
}

// ProcSample is one row of proc_samples.csv.
type ProcSample struct {
	TsMs              int64
	CPUPct            float64
	RSSBytes          uint64
	DiskReadCumBytes  uint64
	DiskWriteCumBytes uint64
}
