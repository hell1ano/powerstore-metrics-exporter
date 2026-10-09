/*
 Copyright (c) 2024-2025 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package bulkClient

type PerformanceMetricsByAppliance struct {
	ApplianceID                 string  `csv:"appliance_id" json:"appliance_id"`
	Timestamp                   string  `csv:"timestamp" json:"timestamp"`
	AvgReadLatency              float64 `csv:"avg_read_latency" json:"avg_read_latency"`
	AvgReadSize                 float64 `csv:"avg_read_size" json:"avg_read_size"`
	AvgLatency                  float64 `csv:"avg_latency" json:"avg_latency"`
	AvgWriteLatency             float64 `csv:"avg_write_latency" json:"avg_write_latency"`
	AvgWriteSize                float64 `csv:"avg_write_size" json:"avg_write_size"`
	AvgReadIops                 float64 `csv:"avg_read_iops" json:"avg_read_iops"`
	AvgReadBandwidth            float64 `csv:"avg_read_bandwidth" json:"avg_read_bandwidth"`
	AvgTotalIops                float64 `csv:"avg_total_iops" json:"avg_total_iops"`
	AvgTotalBandwidth           float64 `csv:"avg_total_bandwidth" json:"avg_total_bandwidth"`
	AvgWriteIops                float64 `csv:"avg_write_iops" json:"avg_write_iops"`
	AvgWriteBandwidth           float64 `csv:"avg_write_bandwidth" json:"avg_write_bandwidth"`
	AvgIoSize                   float64 `csv:"avg_io_size" json:"avg_io_size"`
	AvgIoWorkloadCPUUtilization float64 `csv:"avg_io_workload_cpu_utilization" json:"avg_io_workload_cpu_utilization"`
	AvgMirrorWriteLatency       float64 `csv:"avg_mirror_write_latency" json:"avg_mirror_write_latency"`
	AvgMirrorOverheadLatency    float64 `csv:"avg_mirror_overhead_latency" json:"avg_mirror_overhead_latency"`
	AvgMirrorWriteIops          float64 `csv:"avg_mirror_write_iops" json:"avg_mirror_write_iops"`
	AvgMirrorWriteBandwidth     float64 `csv:"avg_mirror_write_bandwidth" json:"avg_mirror_write_bandwidth"`
}

type PerformanceMetricsByVolume struct {
	VolumeID          string  `csv:"volume_id" json:"volume_id"`
	ApplianceID       string  `csv:"appliance_id" json:"appliance_id"`
	Timestamp         string  `csv:"timestamp" json:"timestamp"`
	AvgReadLatency    float64 `csv:"avg_read_latency" json:"avg_read_latency"`
	AvgReadSize       float64 `csv:"avg_read_size" json:"avg_read_size"`
	AvgLatency        float64 `csv:"avg_latency" json:"avg_latency"`
	AvgWriteLatency   float64 `csv:"avg_write_latency" json:"avg_write_latency"`
	AvgWriteSize      float64 `csv:"avg_write_size" json:"avg_write_size"`
	AvgReadIops       float64 `csv:"avg_read_iops" json:"avg_read_iops"`
	AvgReadBandwidth  float64 `csv:"avg_read_bandwidth" json:"avg_read_bandwidth"`
	AvgTotalIops      float64 `csv:"avg_total_iops" json:"avg_total_iops"`
	AvgTotalBandwidth float64 `csv:"avg_total_bandwidth" json:"avg_total_bandwidth"`
	AvgWriteIops      float64 `csv:"avg_write_iops" json:"avg_write_iops"`
	AvgWriteBandwidth float64 `csv:"avg_write_bandwidth" json:"avg_write_bandwidth"`
	AvgIoSize         float64 `csv:"avg_io_size" json:"avg_io_size"`
}

type PerformanceMetricsByFeEthPort struct {
	FePortID                string  `json:"fe_port_id" csv:"fe_port_id"`
	NodeID                  string  `json:"node_id" csv:"node_id"`
	Timestamp               string  `json:"timestamp" csv:"timestamp"`
	AvgPktRxPs              float64 `json:"avg_pkt_rx_ps" csv:"avg_pkt_rx_ps"`
	AvgPktTxPs              float64 `json:"avg_pkt_tx_ps" csv:"avg_pkt_tx_ps"`
	AvgBytesTxPs            float64 `json:"avg_bytes_tx_ps" csv:"avg_bytes_tx_ps"`
	AvgBytesRxPs            float64 `json:"avg_bytes_rx_ps" csv:"avg_bytes_rx_ps"`
	AvgPktRxNoBufferErrorPs float64 `json:"avg_pkt_rx_no_buffer_error_ps" csv:"avg_pkt_rx_no_buffer_error_ps"`
	AvgPktRxCrcErrorPs      float64 `json:"avg_pkt_rx_crc_error_ps" csv:"avg_pkt_rx_crc_error_ps"`
	AvgPktTxErrorPs         float64 `json:"avg_pkt_tx_error_ps" csv:"avg_pkt_tx_error_ps"`
	MaxPktRxPs              float64 `json:"max_pkt_rx_ps" csv:"max_pkt_rx_ps"`
	MaxPktTxPs              float64 `json:"max_pkt_tx_ps" csv:"max_pkt_tx_ps"`
	MaxBytesTxPs            float64 `json:"max_bytes_tx_ps" csv:"max_bytes_tx_ps"`
	MaxBytesRxPs            float64 `json:"max_bytes_rx_ps" csv:"max_bytes_rx_ps"`
	MaxPktRxNoBufferErrorPs float64 `json:"max_pkt_rx_no_buffer_error_ps" csv:"max_pkt_rx_no_buffer_error_ps"`
	MaxPktRxCrcErrorPs      float64 `json:"max_pkt_rx_crc_error_ps" csv:"max_pkt_rx_crc_error_ps"`
	MaxPktTxErrorPs         float64 `json:"max_pkt_tx_error_ps" csv:"max_pkt_tx_error_ps"`
	ApplianceID             string  `json:"appliance_id" csv:"appliance_id"`
}

type PerformanceMetricsByFeFcPort struct {
	FePortID                   string  `json:"fe_port_id" csv:"fe_port_id"`
	NodeID                     string  `json:"node_id" csv:"node_id"`
	Timestamp                  string  `json:"timestamp" csv:"timestamp"`
	AvgDumpedFramesPs          float64 `json:"avg_dumped_frames_ps" csv:"avg_dumped_frames_ps"`
	AvgLossOfSignalCountPs     float64 `json:"avg_loss_of_signal_count_ps" csv:"avg_loss_of_signal_count_ps"`
	AvgInvalidCrcCountPs       float64 `json:"avg_invalid_crc_count_ps" csv:"avg_invalid_crc_count_ps"`
	AvgLossOfSyncCountPs       float64 `json:"avg_loss_of_sync_count_ps" csv:"avg_loss_of_sync_count_ps"`
	AvgInvalidTxWordCountPs    float64 `json:"avg_invalid_tx_word_count_ps" csv:"avg_invalid_tx_word_count_ps"`
	AvgPrimSeqProtErrCountPs   float64 `json:"avg_prim_seq_prot_err_count_ps" csv:"avg_prim_seq_prot_err_count_ps"`
	AvgLinkFailureCountPs      float64 `json:"avg_link_failure_count_ps" csv:"avg_link_failure_count_ps"`
	MaxDumpedFramesPs          float64 `json:"max_dumped_frames_ps" csv:"max_dumped_frames_ps"`
	MaxLossOfSignalCountPs     float64 `json:"max_loss_of_signal_count_ps" csv:"max_loss_of_signal_count_ps"`
	MaxInvalidCrcCountPs       float64 `json:"max_invalid_crc_count_ps" csv:"max_invalid_crc_count_ps"`
	MaxLossOfSyncCountPs       float64 `json:"max_loss_of_sync_count_ps" csv:"max_loss_of_sync_count_ps"`
	MaxInvalidTxWordCountPs    float64 `json:"max_invalid_tx_word_count_ps" csv:"max_invalid_tx_word_count_ps"`
	MaxPrimSeqProtErrCountPs   float64 `json:"max_prim_seq_prot_err_count_ps" csv:"max_prim_seq_prot_err_count_ps"`
	MaxLinkFailureCountPs      float64 `json:"max_link_failure_count_ps" csv:"max_link_failure_count_ps"`
	ApplianceID                string  `json:"appliance_id" csv:"appliance_id"`
	AvgReadLatency             float64 `json:"avg_read_latency" csv:"avg_read_latency"`
	AvgReadSize                float64 `json:"avg_read_size" csv:"avg_read_size"`
	AvgLatency                 float64 `json:"avg_latency" csv:"avg_latency"`
	AvgWriteLatency            float64 `json:"avg_write_latency" csv:"avg_write_latency"`
	AvgWriteSize               float64 `json:"avg_write_size" csv:"avg_write_size"`
	AvgReadIops                float64 `json:"avg_read_iops" csv:"avg_read_iops"`
	AvgReadBandwidth           float64 `json:"avg_read_bandwidth" csv:"avg_read_bandwidth"`
	AvgTotalIops               float64 `json:"avg_total_iops" csv:"avg_total_iops"`
	AvgTotalBandwidth          float64 `json:"avg_total_bandwidth" csv:"avg_total_bandwidth"`
	AvgWriteIops               float64 `json:"avg_write_iops" csv:"avg_write_iops"`
	AvgWriteBandwidth          float64 `json:"avg_write_bandwidth" csv:"avg_write_bandwidth"`
	MaxAvgReadLatency          float64 `json:"max_avg_read_latency" csv:"max_avg_read_latency"`
	MaxAvgReadSize             float64 `json:"max_avg_read_size" csv:"max_avg_read_size"`
	MaxAvgLatency              float64 `json:"max_avg_latency" csv:"max_avg_latency"`
	MaxAvgWriteLatency         float64 `json:"max_avg_write_latency" csv:"max_avg_write_latency"`
	MaxAvgWriteSize            float64 `json:"max_avg_write_size" csv:"max_avg_write_size"`
	MaxReadIops                float64 `json:"max_read_iops" csv:"max_read_iops"`
	MaxReadBandwidth           float64 `json:"max_read_bandwidth" csv:"max_read_bandwidth"`
	MaxTotalIops               float64 `json:"max_total_iops" csv:"max_total_iops"`
	MaxTotalBandwidth          float64 `json:"max_total_bandwidth" csv:"max_total_bandwidth"`
	MaxWriteIops               float64 `json:"max_write_iops" csv:"max_write_iops"`
	MaxWriteBandwidth          float64 `json:"max_write_bandwidth" csv:"max_write_bandwidth"`
	AvgCurrentLogins           float64 `json:"avg_current_logins" csv:"avg_current_logins"`
	AvgUnalignedWriteBandwidth float64 `json:"avg_unaligned_write_bandwidth" csv:"avg_unaligned_write_bandwidth"`
	AvgUnalignedReadBandwidth  float64 `json:"avg_unaligned_read_bandwidth" csv:"avg_unaligned_read_bandwidth"`
	AvgUnalignedReadIops       float64 `json:"avg_unaligned_read_iops" csv:"avg_unaligned_read_iops"`
	AvgUnalignedWriteIops      float64 `json:"avg_unaligned_write_iops" csv:"avg_unaligned_write_iops"`
	AvgUnalignedBandwidth      float64 `json:"avg_unaligned_bandwidth" csv:"avg_unaligned_bandwidth"`
	AvgUnalignedIops           float64 `json:"avg_unaligned_iops" csv:"avg_unaligned_iops"`
	MaxCurrentLogins           float64 `json:"max_current_logins" csv:"max_current_logins"`
	MaxUnalignedWriteBandwidth float64 `json:"max_unaligned_write_bandwidth" csv:"max_unaligned_write_bandwidth"`
	MaxUnalignedReadBandwidth  float64 `json:"max_unaligned_read_bandwidth" csv:"max_unaligned_read_bandwidth"`
	MaxUnalignedReadIops       float64 `json:"max_unaligned_read_iops" csv:"max_unaligned_read_iops"`
	MaxUnalignedWriteIops      float64 `json:"max_unaligned_write_iops" csv:"max_unaligned_write_iops"`
	MaxUnalignedBandwidth      float64 `json:"max_unaligned_bandwidth" csv:"max_unaligned_bandwidth"`
	MaxUnalignedIops           float64 `json:"max_unaligned_iops" csv:"max_unaligned_iops"`
	AvgIoSize                  float64 `json:"avg_io_size" csv:"avg_io_size"`
	MaxAvgIoSize               float64 `json:"max_avg_io_size" csv:"max_avg_io_size"`
	AvgUnmapIops               float64 `json:"avg_unmap_iops" csv:"avg_unmap_iops"`
	AvgCopyIops                float64 `json:"avg_copy_iops" csv:"avg_copy_iops"`
	AvgZeroIops                float64 `json:"avg_zero_iops" csv:"avg_zero_iops"`
	AvgUnmapBandwidth          float64 `json:"avg_unmap_bandwidth" csv:"avg_unmap_bandwidth"`
	AvgCopyBandwidth           float64 `json:"avg_copy_bandwidth" csv:"avg_copy_bandwidth"`
	AvgZeroBandwidth           float64 `json:"avg_zero_bandwidth" csv:"avg_zero_bandwidth"`
	AvgUnmapIoSize             float64 `json:"avg_unmap_io_size" csv:"avg_unmap_io_size"`
	AvgCopyIoSize              float64 `json:"avg_copy_io_size" csv:"avg_copy_io_size"`
	AvgZeroIoSize              float64 `json:"avg_zero_io_size" csv:"avg_zero_io_size"`
	AvgUnmapLatency            float64 `json:"avg_unmap_latency" csv:"avg_unmap_latency"`
	AvgCopyLatency             float64 `json:"avg_copy_latency" csv:"avg_copy_latency"`
	AvgZeroLatency             float64 `json:"avg_zero_latency" csv:"avg_zero_latency"`
	MaxUnmapIops               float64 `json:"max_unmap_iops" csv:"max_unmap_iops"`
	MaxUnmapBandwidth          float64 `json:"max_unmap_bandwidth" csv:"max_unmap_bandwidth"`
	MaxAvgUnmapIoSize          float64 `json:"max_avg_unmap_io_size" csv:"max_avg_unmap_io_size"`
	MaxAvgUnmapLatency         float64 `json:"max_avg_unmap_latency" csv:"max_avg_unmap_latency"`
	MaxCopyIops                float64 `json:"max_copy_iops" csv:"max_copy_iops"`
	MaxCopyBandwidth           float64 `json:"max_copy_bandwidth" csv:"max_copy_bandwidth"`
	MaxAvgCopyIoSize           float64 `json:"max_avg_copy_io_size" csv:"max_avg_copy_io_size"`
	MaxAvgCopyLatency          float64 `json:"max_avg_copy_latency" csv:"max_avg_copy_latency"`
	MaxZeroIops                float64 `json:"max_zero_iops" csv:"max_zero_iops"`
	MaxZeroBandwidth           float64 `json:"max_zero_bandwidth" csv:"max_zero_bandwidth"`
	MaxAvgZeroIoSize           float64 `json:"max_avg_zero_io_size" csv:"max_avg_zero_io_size"`
	MaxAvgZeroLatency          float64 `json:"max_avg_zero_latency" csv:"max_avg_zero_latency"`
}

type PerformanceMetricsByFileSystem struct {
	AvgBlockWriteIops        float64 `json:"avg_block_write_iops" csv:"avg_block_write_iops"`
	AvgMirrorWriteIops       float64 `json:"avg_mirror_write_iops" csv:"avg_mirror_write_iops"`
	AvgBlockWriteBandwidth   float64 `json:"avg_block_write_bandwidth" csv:"avg_block_write_bandwidth"`
	AvgMirrorWriteBandwidth  float64 `json:"avg_mirror_write_bandwidth" csv:"avg_mirror_write_bandwidth"`
	AvgBlockWriteLatency     float64 `json:"avg_block_write_latency" csv:"avg_block_write_latency"`
	AvgMirrorOverheadLatency float64 `json:"avg_mirror_overhead_latency" csv:"avg_mirror_overhead_latency"`

	FileSystemID       string  `json:"file_system_id" csv:"file_system_id"`
	Timestamp          string  `json:"timestamp" csv:"timestamp"`
	AvgReadIops        float64 `json:"avg_read_iops" csv:"avg_read_iops"`
	AvgWriteIops       float64 `json:"avg_write_iops" csv:"avg_write_iops"`
	AvgTotalIops       float64 `json:"avg_total_iops" csv:"avg_total_iops"`
	MaxReadIops        float64 `json:"max_read_iops" csv:"max_read_iops"`
	MaxWriteIops       float64 `json:"max_write_iops" csv:"max_write_iops"`
	MaxIops            float64 `json:"max_iops" csv:"max_iops"`
	AvgReadBandwidth   float64 `json:"avg_read_bandwidth" csv:"avg_read_bandwidth"`
	AvgWriteBandwidth  float64 `json:"avg_write_bandwidth" csv:"avg_write_bandwidth"`
	AvgTotalBandwidth  float64 `json:"avg_total_bandwidth" csv:"avg_total_bandwidth"`
	MaxReadBandwidth   float64 `json:"max_read_bandwidth" csv:"max_read_bandwidth"`
	MaxWriteBandwidth  float64 `json:"max_write_bandwidth" csv:"max_write_bandwidth"`
	MaxTotalBandwidth  float64 `json:"max_total_bandwidth" csv:"max_total_bandwidth"`
	AvgReadLatency     float64 `json:"avg_read_latency" csv:"avg_read_latency"`
	AvgWriteLatency    float64 `json:"avg_write_latency" csv:"avg_write_latency"`
	AvgLatency         float64 `json:"avg_latency" csv:"avg_latency"`
	MaxAvgReadLatency  float64 `json:"max_avg_read_latency" csv:"max_avg_read_latency"`
	MaxAvgWriteLatency float64 `json:"max_avg_write_latency" csv:"max_avg_write_latency"`
	MaxAvgLatency      float64 `json:"max_avg_latency" csv:"max_avg_latency"`
	AvgReadSize        float64 `json:"avg_read_size" csv:"avg_read_size"`
	AvgWriteSize       float64 `json:"avg_write_size" csv:"avg_write_size"`
	AvgSize            float64 `json:"avg_size" csv:"avg_size"`
	MaxAvgReadSize     float64 `json:"max_avg_read_size" csv:"max_avg_read_size"`
	MaxAvgWriteSize    float64 `json:"max_avg_write_size" csv:"max_avg_write_size"`
	MaxAvgSize         float64 `json:"max_avg_size" csv:"max_avg_size"`
}

type PerformanceMetricsByNasServer struct {
	NasServerID        string  `json:"nas_server_id" csv:"nas_server_id"`
	Timestamp          string  `json:"timestamp" csv:"timestamp"`
	AvgReadIops        float64 `json:"avg_read_iops" csv:"avg_read_iops"`
	AvgWriteIops       float64 `json:"avg_write_iops" csv:"avg_write_iops"`
	AvgTotalIops       float64 `json:"avg_total_iops" csv:"avg_total_iops"`
	MaxReadIops        float64 `json:"max_read_iops" csv:"max_read_iops"`
	MaxWriteIops       float64 `json:"max_write_iops" csv:"max_write_iops"`
	MaxIops            float64 `json:"max_iops" csv:"max_iops"`
	AvgReadBandwidth   float64 `json:"avg_read_bandwidth" csv:"avg_read_bandwidth"`
	AvgWriteBandwidth  float64 `json:"avg_write_bandwidth" csv:"avg_write_bandwidth"`
	AvgTotalBandwidth  float64 `json:"avg_total_bandwidth" csv:"avg_total_bandwidth"`
	MaxReadBandwidth   float64 `json:"max_read_bandwidth" csv:"max_read_bandwidth"`
	MaxWriteBandwidth  float64 `json:"max_write_bandwidth" csv:"max_write_bandwidth"`
	MaxTotalBandwidth  float64 `json:"max_total_bandwidth" csv:"max_total_bandwidth"`
	AvgReadLatency     float64 `json:"avg_read_latency" csv:"avg_read_latency"`
	AvgWriteLatency    float64 `json:"avg_write_latency" csv:"avg_write_latency"`
	AvgLatency         float64 `json:"avg_latency" csv:"avg_latency"`
	MaxAvgReadLatency  float64 `json:"max_avg_read_latency" csv:"max_avg_read_latency"`
	MaxAvgWriteLatency float64 `json:"max_avg_write_latency" csv:"max_avg_write_latency"`
	MaxAvgLatency      float64 `json:"max_avg_latency" csv:"max_avg_latency"`
	AvgReadSize        float64 `json:"avg_read_size" csv:"avg_read_size"`
	AvgWriteSize       float64 `json:"avg_write_size" csv:"avg_write_size"`
	AvgSize            float64 `json:"avg_size" csv:"avg_size"`
	MaxAvgReadSize     float64 `json:"max_avg_read_size" csv:"max_avg_read_size"`
	MaxAvgWriteSize    float64 `json:"max_avg_write_size" csv:"max_avg_write_size"`
	MaxAvgSize         float64 `json:"max_avg_size" csv:"max_avg_size"`
}

type PerformanceMetricsByVg struct {
	Timestamp                   string  `json:"timestamp" csv:"timestamp"`
	VgID                        string  `json:"vg_id" csv:"vg_id"`
	AvgReadLatency              float64 `json:"avg_read_latency" csv:"avg_read_latency"`
	AvgReadSize                 float64 `json:"avg_read_size" csv:"avg_read_size"`
	AvgLatency                  float64 `json:"avg_latency" csv:"avg_latency"`
	AvgWriteLatency             float64 `json:"avg_write_latency" csv:"avg_write_latency"`
	AvgWriteSize                float64 `json:"avg_write_size" csv:"avg_write_size"`
	AvgReadIops                 float64 `json:"avg_read_iops" csv:"avg_read_iops"`
	AvgReadBandwidth            float64 `json:"avg_read_bandwidth" csv:"avg_read_bandwidth"`
	AvgTotalIops                float64 `json:"avg_total_iops" csv:"avg_total_iops"`
	AvgTotalBandwidth           float64 `json:"avg_total_bandwidth" csv:"avg_total_bandwidth"`
	AvgWriteIops                float64 `json:"avg_write_iops" csv:"avg_write_iops"`
	AvgWriteBandwidth           float64 `json:"avg_write_bandwidth" csv:"avg_write_bandwidth"`
	AvgMirrorOverheadLatency    float64 `json:"avg_mirror_overhead_latency" csv:"avg_mirror_overhead_latency"`
	AvgMirrorWriteIops          float64 `json:"avg_mirror_write_iops" csv:"avg_mirror_write_iops"`
	AvgMirrorWriteBandwidth     float64 `json:"avg_mirror_write_bandwidth" csv:"avg_mirror_write_bandwidth"`
	MaxAvgReadLatency           float64 `json:"max_avg_read_latency" csv:"max_avg_read_latency"`
	MaxAvgReadSize              float64 `json:"max_avg_read_size" csv:"max_avg_read_size"`
	MaxAvgLatency               float64 `json:"max_avg_latency" csv:"max_avg_latency"`
	MaxAvgWriteLatency          float64 `json:"max_avg_write_latency" csv:"max_avg_write_latency"`
	MaxAvgWriteSize             float64 `json:"max_avg_write_size" csv:"max_avg_write_size"`
	MaxReadIops                 float64 `json:"max_read_iops" csv:"max_read_iops"`
	MaxReadBandwidth            float64 `json:"max_read_bandwidth" csv:"max_read_bandwidth"`
	MaxTotalIops                float64 `json:"max_total_iops" csv:"max_total_iops"`
	MaxTotalBandwidth           float64 `json:"max_total_bandwidth" csv:"max_total_bandwidth"`
	MaxWriteIops                float64 `json:"max_write_iops" csv:"max_write_iops"`
	MaxWriteBandwidth           float64 `json:"max_write_bandwidth" csv:"max_write_bandwidth"`
	AvgIoSize                   float64 `json:"avg_io_size" csv:"avg_io_size"`
	MaxAvgIoSize                float64 `json:"max_avg_io_size" csv:"max_avg_io_size"`
	MaxAvgMirrorOverheadLatency float64 `json:"max_avg_mirror_overhead_latency" csv:"max_avg_mirror_overhead_latency"`
	MaxMirrorWriteIops          float64 `json:"max_mirror_write_iops" csv:"max_mirror_write_iops"`
	MaxMirrorWriteBandwidth     float64 `json:"max_mirror_write_bandwidth" csv:"max_mirror_write_bandwidth"`
}

type SpaceMetricsByAppliance struct {
	DataPhysicalUsed   int64   `csv:"data_physical_used" json:"data_physical_used"`
	SharedLogicalUsed  int64   `csv:"shared_logical_used" json:"shared_logical_used"`
	ApplianceID        string  `csv:"appliance_id" json:"appliance_id"`
	Timestamp          string  `csv:"timestamp" json:"timestamp"`
	LogicalProvisioned int64   `csv:"logical_provisioned" json:"logical_provisioned"`
	LogicalUsed        int64   `csv:"logical_used" json:"logical_used"`
	PhysicalTotal      int64   `csv:"physical_total" json:"physical_total"`
	PhysicalUsed       int64   `csv:"physical_used" json:"physical_used"`
	EfficiencyRatio    float64 `csv:"efficiency_ratio" json:"efficiency_ratio"`
	DataReduction      float64 `csv:"data_reduction" json:"data_reduction"`
	SnapshotSavings    float64 `csv:"snapshot_savings" json:"snapshot_savings"`
	ThinSavings        float64 `csv:"thin_savings" json:"thin_savings"`
}

type SpaceMetricsByFilesystem struct {
	FileSystemID           string  `json:"file_system_id" csv:"file_system_id"`
	Timestamp              string  `json:"timestamp" csv:"timestamp"`
	LogicalProvisioned     float64 `json:"logical_provisioned" csv:"logical_provisioned"`
	LogicalUsed            float64 `json:"logical_used" csv:"logical_used"`
	ThinSavings            float64 `json:"thin_savings" csv:"thin_savings"`
	LastLogicalProvisioned float64 `json:"last_logical_provisioned" csv:"last_logical_provisioned"`
	LastLogicalUsed        float64 `json:"last_logical_used" csv:"last_logical_used"`
	LastThinSavings        float64 `json:"last_thin_savings" csv:"last_thin_savings"`
	MaxLogicalProvisioned  float64 `json:"max_logical_provisioned" csv:"max_logical_provisioned"`
	MaxLogicalUsed         float64 `json:"max_logical_used" csv:"max_logical_used"`
	MaxThinSavings         float64 `json:"max_thin_savings" csv:"max_thin_savings"`
	UnreducibleData        float64 `json:"unreducible_data" csv:"unreducible_data"`
	DataReduction          float64 `json:"data_reduction" csv:"data_reduction"`
	ReducibleDataReduction float64 `json:"reducible_data_reduction" csv:"reducible_data_reduction"`
}

type WearMetricsByDrive struct {
	DriveID                   string `csv:"drive_id" json:"drive_id"`
	Timestamp                 string `csv:"timestamp" json:"timestamp"`
	PercentEnduranceRemaining int    `csv:"percent_endurance_remaining" json:"percent_endurance_remaining"`
	ApplianceID               string `csv:"appliance_id" json:"appliance_id"`
}
