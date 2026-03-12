package main

import (
	"greenplum-exporter/collector"
	httpserver "greenplum-exporter/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logger "github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
)

/**
 * 参考教程：https://www.cnblogs.com/momoyan/p/9943268.html
 * 官方文档：https://godoc.org/github.com/prometheus/client_golang/prometheus
 * 官方文档：https://gp-docs-cn.github.io/docs/admin_guide/monitoring/monitoring.html
 */

var (
	listenAddress         = kingpin.Flag("web.listen-address", "web endpoint").Default("0.0.0.0:9297").String()
	metricPath            = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	disableDefaultMetrics = kingpin.Flag("disableDefaultMetrics", "do not report default metrics(go metrics and process metrics)").Default("true").Bool()
	webUIEnabled          = kingpin.Flag("web.ui-enabled", "enable web UI for metrics status").Default("true").Bool()
)

// 全局采集器实例
var greenPlumCollector *collector.GreenPlumCollector

var scrapers = map[collector.Scraper]bool{
	collector.NewLocksScraper():         true,
	collector.NewClusterStateScraper():  true,
	collector.NewDatabaseSizeScraper():  true,
	collector.NewConnectionsScraper():   true,
	collector.NewMaxConnScraper():       true,
	collector.NewSegmentScraper():       true,
	collector.NewConnDetailScraper():    true,
	collector.NewUsersScraper():         true,
	collector.NewBgWriterStateScraper(): true,

	// 以下 4 个高级采集器需要 gp_metrics_views 扩展，默认禁用
	// 可通过设置环境变量 ENABLE_ADVANCED_SCRAPERS=true 来启用
	collector.NewSystemScraper():        isEnvTrue("ENABLE_SYSTEM_SCRAPER"),
	collector.NewQueryScraper():         isEnvTrue("ENABLE_QUERY_SCRAPER"),
	collector.NewDynamicMemoryScraper(): isEnvTrue("ENABLE_DYNAMIC_MEMORY_SCRAPER"),
	collector.NewDiskScraper():          isEnvTrue("ENABLE_DISK_SCRAPER"),
}

var gathers prometheus.Gatherers

func main() {
	kingpin.Version("1.1.1")
	kingpin.HelpFlag.Short('h')

	logger.AddFlags(kingpin.CommandLine)
	kingpin.Parse()

	metricsHandleFunc := newHandler(*disableDefaultMetrics, scrapers)

	logger.Warnf("Greenplum exporter is starting and will listening on : %s", *listenAddress)

	// 创建统一的 HTTP 多路复用器
	mux := http.NewServeMux()

	// 注册 Prometheus metrics 路由
	mux.Handle(*metricPath, metricsHandleFunc)

	// 如果启用了 Web UI，注册 Web UI 路由
	if *webUIEnabled {
		// 使用已创建的 global greenPlumCollector 实例
		if greenPlumCollector == nil {
			logger.Error("Greenplum collector is nil, please check initialization")
			return
		}

		// 创建 Web 服务器（不指定地址，只使用路由）
		webServer := httpserver.NewWebServer(greenPlumCollector)

		// 注册 Web UI 路由到 mux
		mux.HandleFunc("/", webServer.HomeHandler)
		mux.HandleFunc("/metrics-status", webServer.MetricsStatusHandler)
		mux.HandleFunc("/collector-info", webServer.CollectorInfoHandler)

		logger.Warnf("Web UI enabled at: http://%s/", *listenAddress)
	}

	// 启动统一的 HTTP 服务器
	logger.Error(http.ListenAndServe(*listenAddress, mux).Error())

	// 阻塞主线程
	select {}
}

// isEnvTrue 检查环境变量是否为 "true" (不区分大小写)
func isEnvTrue(key string) bool {
	val := os.Getenv(key)
	return strings.ToLower(val) == "true"
}

func newHandler(disableDefaultMetrics bool, scrapers map[collector.Scraper]bool) http.HandlerFunc {

	registry := prometheus.NewRegistry()

	enabledScrapers := make([]collector.Scraper, 0, 16)

	for scraper, enable := range scrapers {
		if enable {
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}

	// 创建全局采集器实例
	greenPlumCollector = collector.NewCollector(enabledScrapers)

	registry.MustRegister(greenPlumCollector)

	if disableDefaultMetrics {
		gathers = prometheus.Gatherers{registry}
	} else {
		gathers = prometheus.Gatherers{registry, prometheus.DefaultGatherer}
	}

	handler := promhttp.HandlerFor(gathers, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})

	return handler.ServeHTTP
}
