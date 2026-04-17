package collector

import (
	"sort"
	"sync"
	"time"
)

// ScraperStatus 存储每个 scraper 的状态信息
type ScraperStatus struct {
	Name            string
	Description     string // 采集器描述
	Metrics         string // 负责的指标列表
	LastSuccessTime time.Time
	LastFailureTime time.Time
	LastError       string
	IsRunning       bool
}

// MetricsStore 线程安全地存储所有指标的状态
type MetricsStore struct {
	mu       sync.RWMutex
	statuses map[string]*ScraperStatus
}

// NewMetricsStore 创建新的指标存储
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		statuses: make(map[string]*ScraperStatus),
	}
}

// UpdateSuccess 更新 scraper 的成功状态
func (m *MetricsStore) UpdateSuccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.statuses[name]; !exists {
		m.statuses[name] = &ScraperStatus{Name: name}
	}

	status := m.statuses[name]
	status.LastSuccessTime = time.Now()
	status.IsRunning = false
	status.LastError = ""
}

// UpdateFailure 更新 scraper 的失败状态
func (m *MetricsStore) UpdateFailure(name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.statuses[name]; !exists {
		m.statuses[name] = &ScraperStatus{Name: name}
	}

	status := m.statuses[name]
	status.LastFailureTime = time.Now()
	status.IsRunning = false
	status.LastError = err.Error()
}

// SetRunning 设置 scraper 为运行中状态
func (m *MetricsStore) SetRunning(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.statuses[name]; !exists {
		m.statuses[name] = &ScraperStatus{Name: name}
	}

	status := m.statuses[name]
	status.IsRunning = true
}

// GetStatus 获取指定 scraper 的状态
func (m *MetricsStore) GetStatus(name string) *ScraperStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, exists := m.statuses[name]; exists {
		return status
	}
	return nil
}

// GetAllStatuses 获取所有 scraper 的状态（按名称排序）
func (m *MetricsStore) GetAllStatuses() []*ScraperStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScraperStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		// 创建副本以避免并发访问问题
		statusCopy := &ScraperStatus{
			Name:            status.Name,
			Description:     getScraperDescription(status.Name),
			Metrics:         getScraperMetrics(status.Name),
			LastSuccessTime: status.LastSuccessTime,
			LastFailureTime: status.LastFailureTime,
			LastError:       status.LastError,
			IsRunning:       status.IsRunning,
		}
		result = append(result, statusCopy)
	}

	// 按采集器名称排序，确保顺序一致
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// getScraperDescription 获取采集器的描述信息
func getScraperDescription(name string) string {
	descriptions := map[string]string{
		"cluster_state_scraper":      "集群状态采集器 - 监控 Greenplum 集群的可用性、运行时间、同步状态和配置加载时间",
		"connections_scraper":        "连接数采集器 - 统计当前总连接数、空闲/活跃/运行/等待的连接数",
		"max_connection_scraper":     "最大连接数采集器 - 获取数据库配置的最大连接数限制",
		"segment_scraper":            "Segment 节点采集器 - 监控所有 Segment 节点的状态、角色和模式",
		"users_scraper":              "用户采集器 - 列出所有数据库用户并统计用户总数",
		"locks_scraper":              "锁信息采集器 - 显示当前所有活跃的数据库锁详情",
		"bg_writer_state_scraper":    "后台写入器采集器 - 监控检查点、缓冲区使用等 BG Writer 统计信息",
		"database_size_scraper":      "数据库大小采集器 - 统计每个数据库占用的存储空间大小",
		"connections_detail_scraper": "连接详情采集器 - 按用户和客户端 IP 分组统计连接数详情",
		"system_scraper":             "系统资源采集器 - 监控 CPU、内存、磁盘 IO、网络等系统指标（需要 gp_metrics_views 扩展）",
		"queries_scraper":            "查询统计采集器 - 统计数据库查询活动（需要 gp_metrics_views 扩展）",
		"dynamic_memory_scraper":     "动态内存采集器 - 监控动态内存使用情况（需要 gp_metrics_views 扩展）",
		"disk_scraper":               "磁盘空间采集器 - 监控磁盘空间使用情况（需要 gp_metrics_views 扩展）",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "未知采集器"
}

// getScraperMetrics 获取采集器负责的指标列表
func getScraperMetrics(name string) string {
	metrics := map[string]string{
		"cluster_state_scraper":      "greenplum_cluster_state, greenplum_cluster_uptime, greenplum_cluster_sync, greenplum_cluster_config_last_load_time_seconds",
		"connections_scraper":        "greenplum_cluster_total_connections, greenplum_cluster_idle_connections, greenplum_cluster_active_connections, greenplum_cluster_running_connections, greenplum_cluster_waiting_connections",
		"max_connection_scraper":     "greenplum_cluster_max_connections",
		"segment_scraper":            "greenplum_node_segment_status, greenplum_node_segment_role, greenplum_node_segment_mode, greenplum_node_segment_disk_free_mb_size, greenplum_node_segment_disk_sum_free_mb_size, greenplum_node_segment_disk_sum_device_free_mb_size",
		"users_scraper":              "greenplum_server_users_name_list, greenplum_server_users_total_count",
		"locks_scraper":              "greenplum_server_locks_table_detail",
		"bg_writer_state_scraper":    "greenplum_server_bgwriter_* (11 个 BG Writer 统计指标), greenplum_server_database_hit_cache_percent_rate, greenplum_server_database_transition_commit_percent_rate",
		"database_size_scraper":      "greenplum_node_database_name_mb_size, greenplum_node_database_table_total_count",
		"connections_detail_scraper": "greenplum_cluster_total_connections_per_user, greenplum_cluster_active_connections_per_user, greenplum_cluster_idle_connections_per_user, greenplum_cluster_total_connections_per_client, greenplum_cluster_active_connections_per_client",
		"system_scraper":             "greenplum_node_cpu_* (7 个), greenplum_node_mem_* (8 个), greenplum_node_disk_* (4 个), greenplum_node_net_* (4 个), greenplum_node_swap_* (3 个)",
		"queries_scraper":            "greenplum_cluster_total_queries, greenplum_cluster_running_queries, greenplum_cluster_queued_queries",
		"dynamic_memory_scraper":     "greenplum_node_dynamic_memory_used_mb, greenplum_node_dynamic_memory_available_mb",
		"disk_scraper":               "greenplum_node_fs_total_bytes, greenplum_node_fs_used_bytes, greenplum_node_fs_available_bytes",
	}
	if m, ok := metrics[name]; ok {
		return m
	}
	return "-"
}
