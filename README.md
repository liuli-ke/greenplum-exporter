# Greenplum-exporter

基于 Go 语言为 Greenplum 集成 Prometheus 的监控数据采集器。

**项目地址：**

- Github: https://github.com/liuli-ke/greenplum-exporter
- Gitee: https://gitee.com/liuli-ke/greenplum_exporter

### 一、编译方法

- centos系统下编译

(1) 环境安装
```
wget https://gomirrors.org/dl/go/go1.14.12.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.14.12.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.io,direct
```

(2) 软件编译
```
git clone https://github.com/liulik/greenplum-exporter
cd greenplum-exporter/ && make build
cd bin && ls -l
```

- docker环境下编译

```
git clone https://github.com/liulik/greenplum-exporter
cd greenplum-exporter/
sh docker-build.sh
```

### 二、启动采集器

- centos系统下

```
export GPDB_DATA_SOURCE_URL=postgres://gpadmin:password@10.17.20.11:5432/postgres?sslmode=disable
./greenplum_exporter --web.listen-address="0.0.0.0:9297" --web.telemetry-path="/metrics" --log.level=error
```

- docker运行

```
docker run -d -p 9297:9297 -e GPDB_DATA_SOURCE_URL=postgres://gpadmin:password@10.17.20.11:5432/postgres?sslmode=disable liulik/greenplum-exporter:v1.1.1 
```

注：环境变量GPDB_DATA_SOURCE_URL指定了连接Greenplum数据库的连接串（请使用gpadmin账号连接postgres库），该连接串以postgres://为前缀，具体格式如下：
```
postgres://gpadmin:password@10.17.20.11:5432/postgres?sslmode=disable
postgres://[数据库连接账号，必须为gpadmin]:[账号密码，即gpadmin的密码]@[数据库的IP地址]:[数据库端口号]/[数据库名称，必须为postgres]?[参数名]=[参数值]&[参数名]=[参数值]
```

然后访问监控指标的URL地址： *http://127.0.0.1:9297/metrics*

更多启动参数：

```bash
usage: greenplum-exporter [<flags>]

Flags:
  -h, --help                   Show context-sensitive help (also try --help-long and --help-man).
      --web.listen-address="0.0.0.0:9297"  
                               Address to listen on for web UI and metrics.
      --web.telemetry-path="/metrics"  
                               Path under which to expose Prometheus metrics.
      --disableDefaultMetrics  Disable default metrics (go metrics and process metrics).
      --web.ui-enabled=true    Enable Web UI for viewing scraper status.
      --log.level="info"       Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]
      --log.format="logger:stderr"  
                               Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true"
```

**高级采集器环境变量控制：**

以下 4 个高级采集器需要安装 `gp_metrics_views` 扩展，默认禁用。可通过设置环境变量启用：

```
# 启用系统资源采集器（CPU、内存、磁盘 IO、网络等 26 个指标）
export ENABLE_SYSTEM_SCRAPER=true

# 启用查询统计采集器（查询性能相关指标）
export ENABLE_QUERY_SCRAPER=true

# 启用动态内存采集器（内存上下文指标）
export ENABLE_DYNAMIC_MEMORY_SCRAPER=true

# 启用磁盘采集器（文件系统指标）
export ENABLE_DISK_SCRAPER=true
```

### 三、支持的监控指标

#### 核心采集器（9 个）

| No. | 指标名称 | 类型 | 标签组 | 度量单位 | 指标描述 | 采集器 |
|:----:|:----|:----|:----|:----|:----|:----|
| 1 | greenplum_cluster_state | Gauge | version; master; standby | boolean | 集群可用状态：1→可用；0→不可用 | cluster_state_scraper |
| 2 | greenplum_cluster_uptime | Gauge | - | seconds | 集群启动持续时间 | cluster_state_scraper |
| 3 | greenplum_cluster_sync | Gauge | - | boolean | Master 同步 Standby 状态：1→正常；0→异常 | cluster_state_scraper |
| 4 | greenplum_cluster_max_connections | Gauge | - | int | 最大连接数 | max_conn_scraper |
| 5 | greenplum_cluster_total_connections | Gauge | - | int | 当前总连接数 | connections_scraper |
| 6 | greenplum_cluster_idle_connections | Gauge | - | int | 空闲连接数 | connections_scraper |
| 7 | greenplum_cluster_active_connections | Gauge | - | int | 活跃查询数 | connections_scraper |
| 8 | greenplum_cluster_running_connections | Gauge | - | int | 执行中的查询数 | connections_scraper |
| 9 | greenplum_cluster_waiting_connections | Gauge | - | int | 等待执行的查询数 | connections_scraper |
| 10 | greenplum_node_segment_status | Gauge | hostname; address; dbid; content; preferred_role; port | boolean | Segment 状态：1→up; 0→down | segment_scraper |
| 11 | greenplum_node_segment_role | Gauge | hostname; address; dbid; content; preferred_role; port | int | Segment 角色：1→primary; 2→mirror | segment_scraper |
| 12 | greenplum_node_segment_mode | Gauge | hostname; address; dbid; content; preferred_role; port | int | Segment 模式：1→synced; 2→resyncing; 3→change tracking; 4→not syncing | segment_scraper |
| 13 | greenplum_node_segment_disk_free_mb_size | Gauge | hostname | MB | Segment 主机磁盘剩余空间 | segment_scraper |
| 14 | greenplum_cluster_total_connections_per_client | Gauge | client_addr | int | 每个客户端的连接数 | conn_detail_scraper |
| 15 | greenplum_cluster_idle_connections_per_client | Gauge | client_addr | int | 每个客户端的空闲连接数 | conn_detail_scraper |
| 16 | greenplum_cluster_active_connections_per_client | Gauge | client_addr | int | 每个客户端的活跃连接数 | conn_detail_scraper |
| 17 | greenplum_cluster_total_online_user_count | Gauge | - | int | 在线用户账号数 | users_scraper |
| 18 | greenplum_cluster_total_client_count | Gauge | - | int | 当前所有连接的客户端总数 | users_scraper |
| 19 | greenplum_cluster_total_connections_per_user | Gauge | usename | int | 每个用户的连接数 | users_scraper |
| 20 | greenplum_cluster_idle_connections_per_user | Gauge | usename | int | 每个用户的空闲连接数 | users_scraper |
| 21 | greenplum_cluster_active_connections_per_user | Gauge | usename | int | 每个用户的活跃连接数 | users_scraper |
| 22 | greenplum_cluster_config_last_load_time_seconds | Gauge | - | seconds | 系统配置最后加载时间 | bg_writer_state_scraper |
| 23 | greenplum_node_database_name_mb_size | Gauge | dbname | MB | 每个数据库占用的存储空间 | database_size_scraper |
| 24 | greenplum_node_database_table_total_count | Gauge | dbname | int | 每个数据库内表的总数量 | database_size_scraper |
| 25 | greenplum_server_locks_table_detail | Gauge | pid; datname; usename; locktype; mode; application_name; state | int | 锁详细信息 | locks_scraper |
| 26 | greenplum_server_database_hit_cache_percent_rate | Gauge | - | float | 缓存命中率 | bg_writer_state_scraper |
| 27 | greenplum_server_database_transition_commit_percent_rate | Gauge | - | float | 事务提交率 | bg_writer_state_scraper |
| 28 | greenplum_server_users_name_list | Gauge | usename | int | 用户列表 | users_scraper |
| 29 | greenplum_server_users_total_count | Gauge | - | int | 用户总数 | users_scraper |

#### Exporter 自身指标

| No. | 指标名称 | 类型 | 度量单位 | 指标描述 |
|:----:|:----|:----|:----|:----|
| 1 | greenplum_exporter_total_scraped | Counter | int | 采集总次数 |
| 2 | greenplum_exporter_total_error | Counter | int | 采集错误次数 |
| 3 | greenplum_exporter_scrape_duration_second | Gauge | seconds | 最近一次采集耗时 |

#### 高级采集器（可选，需 gp_metrics_views 扩展）

**1. system_scraper - 系统资源采集器（26 个指标）**

需要环境变量：`ENABLE_SYSTEM_SCRAPER=true`

- CPU 指标（7 个）：`greenplum_node_cpu_*` - user, system, idle, iowait, irq, softirq, steal
- 内存指标（8 个）：`greenplum_node_mem_*` - total, used, free, available, buffers, cached, percent_used
- 磁盘 IO 指标（4 个）：`greenplum_node_disk_*` - read_bytes, write_bytes, read_time, write_time
- 网络指标（4 个）：`greenplum_node_net_*` - recv_bytes, sent_bytes, recv_packets, sent_packets
- 交换空间指标（3 个）：`greenplum_node_swap_*` - total, used, free

**2. query_scraper - 查询统计采集器（3 个指标）**

需要环境变量：`ENABLE_QUERY_SCRAPER=true`

- `greenplum_query_total_count` - 查询总数
- `greenplum_query_mean_exec_time_ms` - 平均执行时间
- `greenplum_query_mean_result_rows` - 平均返回行数

**3. dynamic_memory_scraper - 动态内存采集器（2 个指标）**

需要环境变量：`ENABLE_DYNAMIC_MEMORY_SCRAPER=true`

- `greenplum_dynamic_memory_total_bytes` - 动态内存总量
- `greenplum_dynamic_memory_used_bytes` - 已使用动态内存量

**4. disk_scraper - 磁盘采集器（3 个指标）**

需要环境变量：`ENABLE_DISK_SCRAPER=true`

- `greenplum_disk_total_bytes` - 磁盘总容量
- `greenplum_disk_used_bytes` - 已使用磁盘容量
- `greenplum_disk_free_bytes` - 剩余磁盘容量

### 四、Web UI 页面

本 Exporter 提供内置 Web UI，方便查看采集器状态和指标信息：

- **首页** (`http://localhost:9297/`) - 显示 exporter 概览信息
- **指标状态** (`http://localhost:9297/metrics-status`) - 显示各采集器的运行状态、最后成功/失败时间
- **采集器信息** (`http://localhost:9297/collector-info`) - 显示所有采集器的详细说明和使用方法

### 五、Grafana 仪表盘

- **Dashboard ID**: 13822
- **Dashboard URL**: https://grafana.com/grafana/dashboards/13822
- **配置教程**: https://blog.csdn.net/inrgihc/article/details/108686638

### 六、问题反馈

如果您看到或使用了本工具，或您觉得本工具对您有价值，请为此项目**点个赞**!!

如有问题或建议，请在 GitHub 上提交 Issue 或 Pull Request.
