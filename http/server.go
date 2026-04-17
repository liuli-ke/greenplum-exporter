package http

import (
	"greenplum-exporter/collector"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebServer 结构体用于管理 HTTP 路由
type WebServer struct {
	collector interface {
		GetMetricsStore() *collector.MetricsStore
	}
}

// NewWebServer 创建新的 Web 服务器（只返回路由处理器）
func NewWebServer(collector interface {
	GetMetricsStore() *collector.MetricsStore
}) *WebServer {
	return &WebServer{
		collector: collector,
	}
}

// HomeHandler 首页处理函数
func (s *WebServer) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title":       "Greenplum Exporter",
		"Version":     "1.1.1",
		"Description": "Greenplum Database Prometheus Exporter",
		"Time":        time.Now().Format("2006-01-02 15:04:05"),
	}

	tmpl := template.Must(template.New("home").Parse(homeTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// MetricsStatusHandler 指标状态页面处理函数
func (s *WebServer) MetricsStatusHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":    "指标采集状态",
		"Statuses": []interface{}{},
	}

	// 获取 store 并获取其状态
	if s.collector != nil {
		store := s.collector.GetMetricsStore()
		if store != nil {
			statuses := store.GetAllStatuses()
			data["Statuses"] = statuses
		}
	}

	tmpl := template.Must(template.New("metrics-status").Parse(metricsStatusTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CollectorInfoHandler 采集器信息页面处理函数
func (s *WebServer) CollectorInfoHandler(w http.ResponseWriter, r *http.Request) {
	// 检查环境变量状态
	envStatus := map[string]bool{
		"ENABLE_SYSTEM_SCRAPER":         isEnvTrue("ENABLE_SYSTEM_SCRAPER"),
		"ENABLE_QUERY_SCRAPER":          isEnvTrue("ENABLE_QUERY_SCRAPER"),
		"ENABLE_DYNAMIC_MEMORY_SCRAPER": isEnvTrue("ENABLE_DYNAMIC_MEMORY_SCRAPER"),
		"ENABLE_DISK_SCRAPER":           isEnvTrue("ENABLE_DISK_SCRAPER"),
	}

	data := map[string]interface{}{
		"Title":       "采集器信息",
		"Description": "Greenplum Exporter 采集器详细信息",
		"EnvStatus":   envStatus,
		"Scrapers": []map[string]string{
			{
				"name":    "📊 cluster_state_scraper",
				"desc":    "集群状态采集器 - 监控 Greenplum 集群的可用性、运行时间、同步状态和配置加载时间",
				"metrics": "4 个指标：greenplum_cluster_state, greenplum_cluster_uptime, greenplum_cluster_sync, greenplum_cluster_config_last_load_time_seconds",
			},
			{
				"name":    "🔌 connections_scraper",
				"desc":    "连接数采集器 - 统计当前总连接数、空闲/活跃/运行/等待的连接数",
				"metrics": "5 个指标：greenplum_cluster_total_connections, greenplum_cluster_idle_connections, greenplum_cluster_active_connections, greenplum_cluster_running_connections, greenplum_cluster_waiting_connections",
			},
			{
				"name":    "🔢 max_connection_scraper",
				"desc":    "最大连接数采集器 - 获取数据库配置的最大连接数限制",
				"metrics": "1 个指标：greenplum_cluster_max_connections",
			},
			{
				"name":    "💾 segment_scraper",
				"desc":    "Segment 节点采集器 - 监控所有 Segment 节点的状态、角色、模式和磁盘空间",
				"metrics": "6 个指标：greenplum_node_segment_status, greenplum_node_segment_role, greenplum_node_segment_mode, greenplum_node_segment_disk_free_mb_size, greenplum_node_segment_disk_sum_free_mb_size, greenplum_node_segment_disk_sum_device_free_mb_size",
			},
			{
				"name":    "👥 users_scraper",
				"desc":    "用户采集器 - 列出所有数据库用户并统计用户总数",
				"metrics": "2 个指标：greenplum_server_users_name_list, greenplum_server_users_total_count",
			},
			{
				"name":    "🔒 locks_scraper",
				"desc":    "锁信息采集器 - 显示当前所有活跃的数据库锁详情",
				"metrics": "1 个指标：greenplum_server_locks_table_detail",
			},
			{
				"name":    "✍️ bg_writer_state_scraper",
				"desc":    "后台写入器采集器 - 监控检查点、缓冲区使用等 BG Writer 统计信息和缓存命中率",
				"metrics": "13 个指标：greenplum_server_bgwriter_* (11 个), greenplum_server_database_hit_cache_percent_rate, greenplum_server_database_transition_commit_percent_rate",
			},
			{
				"name":    "📈 database_size_scraper",
				"desc":    "数据库大小采集器 - 统计每个数据库占用的存储空间大小和表数量",
				"metrics": "2 个指标：greenplum_node_database_name_mb_size, greenplum_node_database_table_total_count",
			},
			{
				"name":    "🔍 connections_detail_scraper",
				"desc":    "连接详情采集器 - 按用户和客户端 IP 分组统计连接数详情",
				"metrics": "5 个指标：greenplum_cluster_total_connections_per_user, greenplum_cluster_active_connections_per_user, greenplum_cluster_idle_connections_per_user, greenplum_cluster_total_connections_per_client, greenplum_cluster_active_connections_per_client",
			},
		},
		"AdvancedScrapers": []map[string]string{
			{
				"name":    "⚙️ system_scraper",
				"desc":    "系统资源采集器 - 监控 CPU、内存、磁盘 IO、网络等系统指标",
				"metrics": "26 个系统级指标：greenplum_node_cpu_* (7 个), greenplum_node_mem_* (8 个), greenplum_node_disk_* (4 个), greenplum_node_net_* (4 个), greenplum_node_swap_* (3 个)",
				"depends": "需要 gp_metrics_views 扩展（system_now 视图）",
			},
			{
				"name":    "❓ queries_scraper",
				"desc":    "查询统计采集器 - 统计数据库查询活动",
				"metrics": "3 个查询指标：greenplum_cluster_total_queries, greenplum_cluster_running_queries, greenplum_cluster_queued_queries",
				"depends": "需要 gp_metrics_views 扩展（database_now 视图）",
			},
			{
				"name":    "💾 dynamic_memory_scraper",
				"desc":    "动态内存采集器 - 监控动态内存使用情况",
				"metrics": "2 个内存指标：greenplum_node_dynamic_memory_used_mb, greenplum_node_dynamic_memory_available_mb",
				"depends": "需要 gp_metrics_views 扩展（memory_info 表）",
			},
			{
				"name":    "💿 disk_scraper",
				"desc":    "磁盘空间采集器 - 监控磁盘空间使用情况（比 segment_scraper 更详细）",
				"metrics": "3 个文件系统指标（带 hostname 和 filesystem 标签）：greenplum_node_fs_total_bytes, greenplum_node_fs_used_bytes, greenplum_node_fs_available_bytes",
				"depends": "需要 gp_metrics_views 扩展（diskspace_now 视图）",
			},
		},
	}

	tmpl := template.Must(template.New("collector-info").Parse(collectorInfoTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// isEnvTrue 检查环境变量是否为 "true" (不区分大小写)
func isEnvTrue(key string) bool {
	val := os.Getenv(key)
	return strings.ToLower(val) == "true"
}

// MetricsWebHandler Prometheus 指标 Web 页面处理函数
func (s *WebServer) MetricsWebHandler(w http.ResponseWriter, r *http.Request) {
	// 使用 JavaScript fetch 来动态加载 /metrics 内容
	data := map[string]interface{}{
		"Title": "Prometheus 指标",
	}

	tmpl := template.Must(template.New("metrics-web").Parse(metricsWebTemplate))
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

const homeTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            background-color: #2c3e50;
            color: white;
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 30px;
        }
        .header h1 {
            margin: 0 0 10px 0;
        }
        .info-card {
            background-color: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .nav-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-top: 20px;
        }
        .nav-card {
            background-color: #3498db;
            color: white;
            padding: 25px;
            border-radius: 10px;
            text-decoration: none;
            transition: transform 0.2s, background-color 0.2s;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .nav-card:hover {
            transform: translateY(-5px);
            background-color: #2980b9;
        }
        .nav-card h3 {
            margin: 0 0 10px 0;
        }
        .nav-card p {
            margin: 0;
            opacity: 0.9;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            color: #7f8c8d;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🚀 {{.Title}}</h1>
        <p>版本：{{.Version}} | {{.Description}}</p>
        <p>当前时间：{{.Time}}</p>
    </div>

    <div class="info-card">
        <h2>欢迎使用 Greenplum Exporter</h2>
        <p>这是一个用于监控 Greenplum 数据库的 Prometheus Exporter，提供丰富的指标采集功能。</p>
    </div>

    <h2>导航菜单</h2>
    <div class="nav-grid">
        <a href="/metrics-status" class="nav-card">
            <h3>📊 指标采集状态</h3>
            <p>查看所有指标的最后成功/失败时间、错误信息等状态信息</p>
        </a>
        <a href="/collector-info" class="nav-card">
            <h3>🔧 采集器信息</h3>
            <p>了解所有可用的采集器及其功能说明</p>
        </a>
        <a href="/metrics-web" class="nav-card">
            <h3>📈 Prometheus 指标</h3>
            <p>访问 Prometheus 格式的监控指标数据</p>
        </a>
    </div>

    <div class="footer">
        <p>Greenplum Exporter © 2026</p>
    </div>
</body>
</html>`

const metricsStatusTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <!-- 每 5 秒自动刷新一次页面 -->
    <meta http-equiv="refresh" content="5">
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            background-color: #2c3e50;
            color: white;
            padding: 20px;
            border-radius: 10px;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .nav-links {
            display: flex;
            gap: 15px;
            align-items: center;
        }
        .nav-link {
            color: white;
            text-decoration: none;
            padding: 8px 15px;
            border-radius: 5px;
            transition: background-color 0.2s;
        }
        .nav-link:hover {
            background-color: rgba(255,255,255,0.2);
        }
        .back-link {
            color: white;
            text-decoration: none;
            margin-right: 15px;
            font-weight: bold;
        }
        .back-link:hover {
            opacity: 0.8;
        }
        #countdown {
            background-color: rgba(255,255,255,0.2);
            padding: 5px 10px;
            border-radius: 5px;
            font-weight: bold;
        }
        table {
            width: 100%;
            background-color: white;
            border-radius: 10px;
            overflow: hidden;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-collapse: collapse;
            font-size: 0.9em;
        }
        th, td {
            padding: 15px;
            text-align: left;
            border-bottom: 1px solid #ddd;
            vertical-align: top;
        }
        th {
            background-color: #34495e;
            color: white;
            font-weight: bold;
            white-space: nowrap;
        }
        tr:hover {
            background-color: #f1f1f1;
        }
        .status-running {
            color: #f39c12;
            font-weight: bold;
        }
        .status-success {
            color: #27ae60;
            font-weight: bold;
        }
        .status-error {
            color: #e74c3c;
            font-weight: bold;
        }
        .error-msg {
            color: #e74c3c;
            font-size: 0.9em;
            word-break: break-all;
        }
        .timestamp {
            color: #7f8c8d;
            font-size: 0.9em;
        }
        /* 刷新高亮动画 */
        @keyframes highlight {
            0% { background-color: #fff3cd; }
            100% { background-color: white; }
        }
        .flash-update {
            animation: highlight 1s ease-out;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="nav-links">
            <a href="/" class="back-link">🏠 首页</a>
            <a href="/metrics-status" class="nav-link">📊 指标状态</a>
            <a href="/collector-info" class="nav-link">🔧 采集器信息</a>
            <a href="/metrics-web" class="nav-link">📈 Prometheus 指标</a>
        </div>
        <div class="auto-refresh-info">
            🔄 自动刷新中 (<span id="countdown">5</span>秒后刷新)
        </div>
    </div>

    <table id="statusTable">
        <thead>
            <tr>
                <th>采集器名称</th>
                <th>状态</th>
                <th>最后成功时间</th>
                <th>最后失败时间</th>
                <th>错误信息</th>
            </tr>
        </thead>
        <tbody>
            {{range .Statuses}}
            <tr>
                <td>{{.Name}}</td>
                <td>
                    {{if .IsRunning}}
                        <span class="status-running">⏳ 运行中...</span>
                    {{else if .LastError}}
                        <span class="status-error">❌ 失败</span>
                    {{else if .LastSuccessTime.After .LastFailureTime}}
                        <span class="status-success">✅ 成功</span>
                    {{else if not .LastSuccessTime.IsZero}}
                        <span class="status-success">✅ 成功</span>
                    {{else}}
                        <span class="status-error">❌ 等待首次采集</span>
                    {{end}}
                </td>
                <td class="timestamp">
                    {{if not .LastSuccessTime.IsZero}}
                        {{.LastSuccessTime.Format "2006-01-02 15:04:05"}}
                    {{else}}
                        -
                    {{end}}
                </td>
                <td class="timestamp">
                    {{if not .LastFailureTime.IsZero}}
                        {{.LastFailureTime.Format "2006-01-02 15:04:05"}}
                    {{else}}
                        -
                    {{end}}
                </td>
                <td class="error-msg">
                    {{if .LastError}}
                        ⚠️ {{.LastError}}
                    {{else}}
                        -
                    {{end}}
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
    
    <script>
        // 倒计时显示
        let countdown = 5;
        const countdownEl = document.getElementById('countdown');
        
        setInterval(() => {
            countdown--;
            if (countdown <= 0) {
                countdown = 5;
            }
            countdownEl.textContent = countdown;
        }, 1000);
        
        // 页面加载完成后给表格行添加动画效果
        window.addEventListener('load', () => {
            const rows = document.querySelectorAll('#statusTable tbody tr');
            rows.forEach((row, index) => {
                setTimeout(() => {
                    row.classList.add('flash-update');
                    setTimeout(() => row.classList.remove('flash-update'), 1000);
                }, index * 100);
            });
        });
    </script>
</body>
</html>`

const collectorInfoTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            background-color: #2c3e50;
            color: white;
            padding: 20px;
            border-radius: 10px;
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .nav-links {
            display: flex;
            gap: 15px;
            align-items: center;
        }
        .nav-link {
            color: white;
            text-decoration: none;
            padding: 8px 15px;
            border-radius: 5px;
            transition: background-color 0.2s;
        }
        .nav-link:hover {
            background-color: rgba(255,255,255,0.2);
        }
        .back-link {
            color: white;
            text-decoration: none;
            font-weight: bold;
        }
        .back-link:hover {
            opacity: 0.8;
        }
        .info-card {
            background-color: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .scraper-list {
            list-style: none;
            padding: 0;
        }
        .scraper-item {
            background-color: white;
            padding: 15px;
            margin-bottom: 10px;
            border-radius: 5px;
            border-left: 4px solid #3498db;
            box-shadow: 0 2px 4px rgba(0,0,0,0.05);
        }
        .scraper-item:hover {
            border-left-color: #2ecc71;
            transform: translateX(5px);
            transition: all 0.2s;
        }
        .env-status {
            display: inline-block;
            padding: 3px 8px;
            border-radius: 3px;
            font-size: 0.85em;
            font-weight: bold;
            margin-left: 10px;
        }
        .env-enabled {
            background-color: #27ae60;
            color: white;
        }
        .env-disabled {
            background-color: #95a5a6;
            color: white;
        }
        .usage-guide {
            background-color: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 15px;
            margin-top: 20px;
            border-radius: 5px;
        }
        .usage-guide h3 {
            margin-top: 0;
            color: #856404;
        }
        .usage-guide code {
            background-color: #f8f9fa;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
        }
        .usage-guide pre {
            background-color: #f8f9fa;
            padding: 10px;
            border-radius: 5px;
            overflow-x: auto;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="nav-links">
            <a href="/" class="back-link">🏠 首页</a>
            <a href="/metrics-status" class="nav-link">📊 指标状态</a>
            <a href="/collector-info" class="nav-link">🔧 采集器信息</a>
            <a href="/metrics-web" class="nav-link">📈 Prometheus 指标</a>
        </div>
    </div>

    <div class="info-card">
        <h2>{{.Description}}</h2>
        <p>以下是 Greenplum Exporter 支持的所有采集器，按功能分类：</p>
        <ul style="margin-top: 15px; padding-left: 20px;">
            <li><strong>核心监控（9 个）：</strong> cluster_state, connections, max_connection, segment, users, locks, bg_writer_state, database_size, connections_detail</li>
            <li><strong>扩展监控（4 个，需 gp_metrics_views）：</strong> system, queries, dynamic_memory, disk</li>
        </ul>
    </div>

    <div class="info-card">
        <h2>📦 核心采集器（已启用）</h2>
        {{range .Scrapers}}
        <div class="scraper-item" style="margin-bottom: 15px;">
            <p style="margin: 0 0 10px 0; font-weight: bold; font-size: 1.1em;">{{index . "name"}}</p>
            <p style="margin: 0 0 8px 0; color: #555;">{{index . "desc"}}</p>
            <p style="margin: 0;"><strong style="color: #27ae60;">📊 指标列表：</strong> {{index . "metrics"}}</p>
        </div>
        {{end}}
    </div>

    <div class="info-card">
        <h2>⚙️ 高级采集器（需 gp_metrics_views 扩展）</h2>
        <p style="color: #e67e22; font-weight: bold;">
            ℹ️ 这些采集器需要安装 gp_metrics_views 扩展才能正常工作
        </p>
        
        {{range .AdvancedScrapers}}
        <div class="scraper-item" style="margin-bottom: 15px;">
            <p style="margin: 0 0 10px 0; font-weight: bold; font-size: 1.1em;">{{index . "name"}}</p>
            <p style="margin: 0 0 8px 0; color: #555;">{{index . "desc"}}</p>
            <p style="margin: 0 0 8px 0;"><strong style="color: #27ae60;">📊 指标列表：</strong> {{index . "metrics"}}</p>
            <p style="margin: 0; color: #e74c3c;"><strong>⚠️ 依赖：</strong> {{index . "depends"}}</p>
        </div>
        {{end}}

        <div class="usage-guide">
            <h3>🔧 如何启用高级采集器</h3>
            <p><strong>步骤 1：安装 gp_metrics_views 扩展</strong></p>
            <pre><code>psql -h &lt;master_host&gt; -U gpadmin -d &lt;database_name&gt; -c "CREATE EXTENSION IF NOT EXISTS gp_metrics_views;"</code></pre>
            
            <p style="margin-top: 15px;"><strong>步骤 2：设置环境变量启用采集器</strong></p>
            <p>在启动 Exporter 之前，设置以下环境变量（根据需要选择）：</p>
            <ul>
                <li>
                    <code>ENABLE_SYSTEM_SCRAPER=true</code>
                    {{if index .EnvStatus "ENABLE_SYSTEM_SCRAPER"}}
                        <span class="env-status env-enabled">✅ 已启用</span>
                    {{else}}
                        <span class="env-status env-disabled">❌ 未启用</span>
                    {{end}}
                </li>
                <li>
                    <code>ENABLE_QUERY_SCRAPER=true</code>
                    {{if index .EnvStatus "ENABLE_QUERY_SCRAPER"}}
                        <span class="env-status env-enabled">✅ 已启用</span>
                    {{else}}
                        <span class="env-status env-disabled">❌ 未启用</span>
                    {{end}}
                </li>
                <li>
                    <code>ENABLE_DYNAMIC_MEMORY_SCRAPER=true</code>
                    {{if index .EnvStatus "ENABLE_DYNAMIC_MEMORY_SCRAPER"}}
                        <span class="env-status env-enabled">✅ 已启用</span>
                    {{else}}
                        <span class="env-status env-disabled">❌ 未启用</span>
                    {{end}}
                </li>
                <li>
                    <code>ENABLE_DISK_SCRAPER=true</code>
                    {{if index .EnvStatus "ENABLE_DISK_SCRAPER"}}
                        <span class="env-status env-enabled">✅ 已启用</span>
                    {{else}}
                        <span class="env-status env-disabled">❌ 未启用</span>
                    {{end}}
                </li>
            </ul>

            <p style="margin-top: 15px;"><strong>步骤 3：启动 Exporter</strong></p>
            <pre><code># Linux/Mac 示例
export ENABLE_SYSTEM_SCRAPER=true
export ENABLE_QUERY_SCRAPER=true
./greenplum_exporter

# Windows PowerShell 示例
$env:ENABLE_SYSTEM_SCRAPER="true"
$env:ENABLE_QUERY_SCRAPER="true"
.\greenplum_exporter.exe

# 或者一次性设置所有
export ENABLE_SYSTEM_SCRAPER=true ENABLE_QUERY_SCRAPER=true ENABLE_DYNAMIC_MEMORY_SCRAPER=true ENABLE_DISK_SCRAPER=true
./greenplum_exporter</code></pre>

            <p style="margin-top: 15px;"><strong>验证是否生效：</strong></p>
            <p>访问 <code>/metrics</code> 页面，检查是否有以下新增指标：</p>
            <ul>
                <li><strong>System Scraper:</strong> greenplum_node_cpu_*, greenplum_node_mem_*, greenplum_node_disk_*, greenplum_node_net_*</li>
                <li><strong>Query Scraper:</strong> greenplum_cluster_total_queries, greenplum_cluster_running_queries</li>
                <li><strong>Dynamic Memory Scraper:</strong> greenplum_node_dynamic_memory_used_mb</li>
                <li><strong>Disk Scraper:</strong> greenplum_node_fs_total_bytes, greenplum_node_fs_used_bytes</li>
            </ul>
        </div>
    </div>
</body>
</html>`

const metricsWebTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f5f5f5;
        }
        .header {
            background-color: #2c3e50;
            color: white;
            padding: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .nav-links {
            display: flex;
            gap: 15px;
            align-items: center;
        }
        .nav-link {
            color: white;
            text-decoration: none;
            padding: 8px 15px;
            border-radius: 5px;
            transition: background-color 0.2s;
        }
        .nav-link:hover {
            background-color: rgba(255,255,255,0.2);
        }
        .back-link {
            color: white;
            text-decoration: none;
            font-weight: bold;
        }
        .back-link:hover {
            opacity: 0.8;
        }
        .content {
            padding: 20px;
            max-width: 100%;
            overflow-x: auto;
        }
        .metrics-container {
            background-color: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .metrics-text {
            font-family: 'Courier New', monospace;
            font-size: 13px;
            line-height: 1.6;
            white-space: pre-wrap;
            word-wrap: break-word;
            color: #333;
        }
        .loading {
            text-align: center;
            padding: 50px;
            color: #7f8c8d;
            font-size: 18px;
        }
        .error {
            text-align: center;
            padding: 50px;
            color: #e74c3c;
            font-size: 18px;
        }
        .refresh-btn {
            background-color: #3498db;
            color: white;
            border: none;
            padding: 8px 15px;
            border-radius: 5px;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        .refresh-btn:hover {
            background-color: #2980b9;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="nav-links">
            <a href="/" class="back-link">🏠 首页</a>
            <a href="/metrics-status" class="nav-link">📊 指标状态</a>
            <a href="/collector-info" class="nav-link">🔧 采集器信息</a>
            <a href="/metrics-web" class="nav-link">📈 Prometheus 指标</a>
        </div>
        <button class="refresh-btn" onclick="loadMetrics()">🔄 刷新</button>
    </div>

    <div class="content">
        <div class="metrics-container">
            <div id="metrics-content" class="loading">加载中...</div>
        </div>
    </div>

    <script>
        async function loadMetrics() {
            const contentDiv = document.getElementById('metrics-content');
            contentDiv.className = 'loading';
            contentDiv.textContent = '加载中...';
            
            try {
                const response = await fetch('/metrics');
                if (!response.ok) {
                    throw new Error('HTTP error! status: ' + response.status);
                }
                const text = await response.text();
                contentDiv.className = 'metrics-text';
                contentDiv.textContent = text;
            } catch (error) {
                contentDiv.className = 'error';
                contentDiv.textContent = '加载失败: ' + error.message;
            }
        }

        // 页面加载时自动获取数据
        window.onload = loadMetrics;
    </script>
</body>
</html>`
