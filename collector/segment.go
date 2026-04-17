package collector

import (
	"context"
	"database/sql"
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
	"strings"
	"time"
)

/**
 * Segment的抓取器
 * 抓取参数包括：节点状态status、最优角色运转preferred_role、正在重新同步mode、磁盘剩余空间disk_free等参数
 */

const (
	segmentConfigSql_V6 = `select dbid,content,role,preferred_role,mode,status,port,hostname,address,datadir from gp_segment_configuration;`
	segmentConfigSql_V5 = `select dbid,content,role,preferred_role,mode,status,port,hostname,address,null as datadir from gp_segment_configuration;`

	segmentDiskFreeSizeSql = `SELECT dfhostname as segment_hostname,sum(dfspace)/count(dfspace)/(1024*1024) as segment_disk_free_gb from gp_toolkit.gp_disk_free GROUP BY dfhostname;`

	segmentDiskSumFreeSizeSql = `SELECT dfhostname as segment_hostname,sum(dfspace)/(1024*1024) as segment_disk_sum_free_mb from gp_toolkit.gp_disk_free GROUP BY dfhostname;`

	segmentDiskDeviceFreeSizeSql = `SELECT dfhostname as segment_hostname,dfdevice as device,dfspace/(1024*1024) as segment_disk_device_free_mb from gp_toolkit.gp_disk_free;`
)

var (
	statusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_status"),
		"UP(1) if the segment is running, DOWN(0) if the segment has failed or is unreachable",
		[]string{"hostname", "address", "dbid", "content", "preferred_role", "port", "data_dir"}, nil,
	)

	roleDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_role"),
		"The segment's current role, either primary or mirror",
		[]string{"hostname", "address", "dbid", "content", "preferred_role", "port", "data_dir"}, nil,
	)

	modeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_mode"),
		"The replication status for the segment",
		[]string{"hostname", "address", "dbid", "content", "preferred_role", "port", "data_dir"}, nil,
	)

	segmentDiskFreeSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_disk_free_mb_size"), //指标的名称
		"Total MB size of each segment node free size of disk in the file system",     //帮助信息，显示在指标的上面作为注释
		[]string{"hostname"}, //定义的label名称数组
		nil,                  //定义的Labels
	)

	segmentDiskSumFreeSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_disk_sum_free_mb_size"),
		"Total sum MB size of each segment node free disk space across all devices",
		[]string{"hostname"},
		nil,
	)

	segmentDiskDeviceFreeSizeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "segment_disk_sum_device_free_mb_size"),
		"Free MB size of each segment node per device",
		[]string{"hostname", "device"},
		nil,
	)
)

func NewSegmentScraper() Scraper {
	return segmentScraper{}
}

type segmentScraper struct{}

func (segmentScraper) Name() string {
	return "segment_scraper"
}

func (segmentScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	errU := scrapeSegmentConfig(db, ch, ver)
	errC := scrapeSegmentDiskFree(db, ch)
	errS := scrapeSegmentDiskSumFree(db, ch)
	errD := scrapeSegmentDiskDeviceFree(db, ch)

	return combineErr(errC, errU, errS, errD)
}

func scrapeSegmentConfig(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)

	defer cancel()

	querySql := segmentConfigSql_V6
	if ver < 6 {
		querySql = segmentConfigSql_V5
	}

	logger.Infof("Query Database: %s", querySql)
	rows, err := db.QueryContext(ctx, querySql)

	if err != nil {
		return err
	}

	defer rows.Close()

	errs := make([]error, 0)

	for rows.Next() {
		var dbID, content, role, preferredRole, mode, status, hostname, address, port string
		var rp sql.NullString

		err = rows.Scan(&dbID, &content, &role, &preferredRole, &mode, &status, &port, &hostname, &address, &rp)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(statusDesc, prometheus.GaugeValue, getStatus(status), hostname, address, dbID, content, preferredRole, port, rp.String)
		ch <- prometheus.MustNewConstMetric(roleDesc, prometheus.GaugeValue, getRole(role), hostname, address, dbID, content, preferredRole, port, rp.String)
		ch <- prometheus.MustNewConstMetric(modeDesc, prometheus.GaugeValue, getMode(mode), hostname, address, dbID, content, preferredRole, port, rp.String)
	}

	return combineErr(errs...)
}

func scrapeSegmentDiskFree(db *sql.DB, ch chan<- prometheus.Metric) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)

	defer cancel()

	logger.Infof("Query Database: %s", segmentDiskFreeSizeSql)
	rows, err := db.QueryContext(ctx, segmentDiskFreeSizeSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	errs := make([]error, 0)

	for rows.Next() {
		var hostName string
		var mbSize float64

		err := rows.Scan(&hostName, &mbSize)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		hostName = strings.TrimSpace(hostName)
		ch <- prometheus.MustNewConstMetric(segmentDiskFreeSizeDesc, prometheus.GaugeValue, mbSize, hostName)
	}

	return combineErr(errs...)
}

func scrapeSegmentDiskSumFree(db *sql.DB, ch chan<- prometheus.Metric) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)

	defer cancel()

	logger.Infof("Query Database: %s", segmentDiskSumFreeSizeSql)
	rows, err := db.QueryContext(ctx, segmentDiskSumFreeSizeSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	errs := make([]error, 0)

	for rows.Next() {
		var hostName string
		var mbSize float64

		err := rows.Scan(&hostName, &mbSize)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		hostName = strings.TrimSpace(hostName)
		ch <- prometheus.MustNewConstMetric(segmentDiskSumFreeSizeDesc, prometheus.GaugeValue, mbSize, hostName)
	}

	return combineErr(errs...)
}

func scrapeSegmentDiskDeviceFree(db *sql.DB, ch chan<- prometheus.Metric) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)

	defer cancel()

	logger.Infof("Query Database: %s", segmentDiskDeviceFreeSizeSql)
	rows, err := db.QueryContext(ctx, segmentDiskDeviceFreeSizeSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	errs := make([]error, 0)

	for rows.Next() {
		var hostName, device string
		var mbSize float64

		err := rows.Scan(&hostName, &device, &mbSize)

		if err != nil {
			errs = append(errs, err)
			continue
		}

		hostName = strings.TrimSpace(hostName)
		device = strings.TrimSpace(device)
		ch <- prometheus.MustNewConstMetric(segmentDiskDeviceFreeSizeDesc, prometheus.GaugeValue, mbSize, hostName, device)
	}

	return combineErr(errs...)
}
