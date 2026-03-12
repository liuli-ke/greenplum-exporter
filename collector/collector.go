package collector

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
	"greenplum-exporter/stopwatch"
	"os"
	"sync"
	"time"
)

const verMajorSql = `select (select regexp_matches((select (select regexp_matches((select version()), 'Greenplum Database \d{1,}\.\d{1,}\.\d{1,}'))[1] as version), '\d{1,}'))[1];`

/**
 *  采集器结构体
 */
type GreenPlumCollector struct {
	mu sync.Mutex

	db       *sql.DB
	ver      int
	metrics  *ExporterMetrics
	scrapers []Scraper
	store    *MetricsStore

	// 后台定时采集相关
	metricsCache   []prometheus.Metric // 缓存的指标数据
	metricsCacheMu sync.RWMutex        // 缓存读写锁
	lastScrapeTime time.Time           // 最后采集时间
	scrapeInterval time.Duration       // 采集间隔
	stopChan       chan struct{}       // 停止信号
	isRunning      bool                // 后台采集是否运行中
}

/**
 *  函数：NewCollector
 *  功能：采集器的生成工厂方法
 */
func NewCollector(enabledScrapers []Scraper) *GreenPlumCollector {
	c := &GreenPlumCollector{
		metrics:        NewMetrics(),
		scrapers:       enabledScrapers,
		store:          NewMetricsStore(),
		metricsCache:   make([]prometheus.Metric, 0),
		scrapeInterval: 10 * time.Second, // 默认 10 秒采集一次
		stopChan:       make(chan struct{}),
	}

	// 启动后台定时采集
	c.startBackgroundScraping()

	return c
}

// startBackgroundScraping 启动后台定时采集
func (c *GreenPlumCollector) startBackgroundScraping() {
	c.isRunning = true
	go func() {
		ticker := time.NewTicker(c.scrapeInterval)
		defer ticker.Stop()

		// 立即执行一次采集
		c.scrapeOnce()

		for {
			select {
			case <-ticker.C:
				c.scrapeOnce()
			case <-c.stopChan:
				logger.Info("background scraping stopped")
				c.isRunning = false
				return
			}
		}
	}()

	logger.Infof("background scraping started with interval: %v", c.scrapeInterval)
}

// scrapeOnce 执行一次采集并缓存结果
func (c *GreenPlumCollector) scrapeOnce() {
	start := time.Now()
	watch := stopwatch.New("scrape")

	// 检查连接
	c.mu.Lock()
	err := c.checkGreenPlumConn()
	if err != nil {
		c.mu.Unlock()
		c.metrics.totalError.Inc()
		c.metrics.greenPlumUp.Set(0)
		logger.Errorf("check database connection failed, error:%v", err)
		c.updateCache(nil, start, err, watch)
		return
	}
	c.mu.Unlock()

	// 执行异步采集
	metrics := c.scrapeAsync(start, watch)

	logger.Infof("scrapeOnce completed: collected %d metrics", len(metrics))

	// 更新缓存
	c.updateCache(metrics, start, nil, watch)
}

// updateCache 更新指标缓存
func (c *GreenPlumCollector) updateCache(metrics []prometheus.Metric, start time.Time, err error, watch *stopwatch.StopWatch) {
	c.metricsCacheMu.Lock()
	defer c.metricsCacheMu.Unlock()

	logger.Infof("updateCache called with %d metrics", len(metrics))

	// 更新缓存 - 保留所有 scraper 的指标
	if metrics != nil && len(metrics) > 0 {
		c.metricsCache = metrics
		logger.Infof("cache updated with %d metrics", len(c.metricsCache))
	} else {
		logger.Warnf("no metrics to cache, keeping old cache")
	}

	c.lastScrapeTime = time.Now()

	// 更新采集器自身指标
	if err == nil {
		c.metrics.greenPlumUp.Set(1)
	} else {
		c.metrics.greenPlumUp.Set(0)
	}

	if watch != nil {
		duration := time.Since(start).Seconds()
		c.metrics.scrapeDuration.Set(duration)
		logger.Infof("scrape duration: %.3f seconds", duration)
	}
}

// Stop 停止后台采集
func (c *GreenPlumCollector) Stop() {
	if c.isRunning {
		close(c.stopChan)
	}
}

/**
 * 接口：Collect
 * 功能：从缓存中快速返回指标数据
 */
func (c *GreenPlumCollector) Collect(ch chan<- prometheus.Metric) {
	// 从缓存读取，快速响应
	c.metricsCacheMu.RLock()
	cacheSize := len(c.metricsCache)
	c.metricsCacheMu.RUnlock()

	logger.Debugf("Collect called, cache size: %d", cacheSize)

	c.metricsCacheMu.RLock()
	defer c.metricsCacheMu.RUnlock()

	// 发送缓存的 scraper 指标
	for _, metric := range c.metricsCache {
		ch <- metric
	}

	// 发送采集器自身指标
	ch <- c.metrics.totalScraped
	ch <- c.metrics.totalError
	ch <- c.metrics.scrapeDuration
	ch <- c.metrics.greenPlumUp

	logger.Infof("Collect sent %d scraper metrics + 4 exporter metrics", cacheSize)
}

/**
 * 接口：Describe
 * 功能：传递结构体中的指标描述符到 channel
 */
func (c *GreenPlumCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics.greenPlumUp.Desc()
	ch <- c.metrics.scrapeDuration.Desc()
	ch <- c.metrics.totalScraped.Desc()
	ch <- c.metrics.totalError.Desc()
}

/**
 * 函数：scrapeAsync
 * 功能：异步并发执行数据抓取，返回采集的指标列表
 */
func (c *GreenPlumCollector) scrapeAsync(start time.Time, watch *stopwatch.StopWatch) []prometheus.Metric {
	// 检查并与 Greenplum 建立连接
	c.metrics.totalScraped.Inc()
	watch.MustStart("check connections")
	err := c.checkGreenPlumConn()
	watch.MustStop()
	if err != nil {
		c.metrics.totalError.Inc()
		c.metrics.scrapeDuration.Set(time.Since(start).Seconds())
		c.metrics.greenPlumUp.Set(0)

		logger.Errorf("check database connection failed, error:%v", err)

		return nil
	}

	logger.Info("check connections ok!")
	c.metrics.greenPlumUp.Set(1)

	// 使用 goroutine 并发执行所有 scraper
	var wg sync.WaitGroup
	// 使用带缓冲的 channel 来接收所有 scraper 的指标
	metricCh := make(chan prometheus.Metric, len(c.scrapers)*20)

	// 重要：在启动任何 goroutine 之前，预先为所有 scraper 增加 WaitGroup 计数器
	wg.Add(len(c.scrapers))

	// 为每个 scraper 启动一个 goroutine
	for _, scraper := range c.scrapers {
		go func(scraper Scraper) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("scraper %s panic: %v", scraper.Name(), r)
					c.store.UpdateFailure(scraper.Name(), fmt.Errorf("panic: %v", r))
				}
			}()

			logger.Info("#### scraping start : " + scraper.Name())
			c.store.SetRunning(scraper.Name())

			scraperStart := time.Now()
			err := scraper.Scrape(c.db, metricCh, c.ver)

			if err != nil {
				logger.Errorf("get metrics for scraper:%s failed, error:%v", scraper.Name(), err.Error())
				c.store.UpdateFailure(scraper.Name(), err)
			} else {
				c.store.UpdateSuccess(scraper.Name())
			}

			logger.Infof("#### scraping end : %s, elapsed: %v", scraper.Name(), time.Since(scraperStart))
		}(scraper)
	}

	// 启动一个独立的 goroutine 来等待所有 scraper 完成并关闭 channel
	go func() {
		wg.Wait()
		logger.Info("all scrapers completed, closing metric channel")
		close(metricCh)
	}()

	// 收集所有指标到 slice
	var metrics []prometheus.Metric
	for metric := range metricCh {
		metrics = append(metrics, metric)
	}

	logger.Infof("scrapeAsync collected %d metrics", len(metrics))
	c.metrics.scrapeDuration.Set(time.Since(start).Seconds())
	logger.Info(fmt.Sprintf("prometheus scraped greenplum exporter successfully at %v, detail elapsed:%s", time.Now(), watch.PrettyPrint()))

	return metrics
}

/**
 * 函数：GetMetricsStore
 * 功能：获取指标存储
 */
func (c *GreenPlumCollector) GetMetricsStore() *MetricsStore {
	return c.store
}

/**
 * 函数：checkGreenPlumConn
 * 功能：检查Greenplum数据库的连接
 */
func (c *GreenPlumCollector) checkGreenPlumConn() (err error) {
	if c.db == nil {
		return c.getGreenPlumConnection()
	}

	if err = c.getGreenplumMajorVersion(c.db); err == nil {
		return nil
	} else {
		_ = c.db.Close()
		c.db = nil
		return c.getGreenPlumConnection()
	}
}

/**
 * 函数：getGreenPlumConnection
 * 功能：获取Greenplum数据库的连接
 */
func (c *GreenPlumCollector) getGreenPlumConnection() error {
	//使用PostgreSQL的驱动连接数据库，可参考如下教程：
	//参考：https://blog.csdn.net/u010412301/article/details/85037685
	dataSourceName := os.Getenv("GPDB_DATA_SOURCE_URL")

	db, err := sql.Open("postgres", dataSourceName)

	if err != nil {
		return err
	}

	if err = c.getGreenplumMajorVersion(db); err != nil {
		_ = db.Close()
		return err
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)

	c.db = db

	return nil
}

/**
 * 函数：getGreenplumMajorVersion
 * 功能：获取Greenplum数据库的主版本号
 */
func (c *GreenPlumCollector) getGreenplumMajorVersion(db *sql.DB) error {
	err := db.Ping()

	if err != nil {
		return err
	}

	rows, err := db.Query(verMajorSql)

	if err != nil {
		return err
	}

	for rows.Next() {
		var verMajor int
		errC := rows.Scan(&verMajor)
		if errC != nil {
			return errC
		}

		c.ver = verMajor
	}

	defer rows.Close()

	return nil
}
