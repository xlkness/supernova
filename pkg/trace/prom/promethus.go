package prom

import (
	"fmt"
	"sync/atomic"

	ginPprof "github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"joynova.com/library/supernova/pkg/jweb"
)

func NewCounter(name string) *PromeCounterStatMgr {
	mgr := new(PromeCounterStatMgr)
	mgr.promeStatMgr = newPromeStatMgr(name, func(defaultLabels, labels []string) prometheus.Collector {
		return newCounterVec(name, defaultLabels, labels)
	})
	return mgr
}

func NewGauge(name string) *PromeGaugeStatMgr {
	mgr := new(PromeGaugeStatMgr)
	mgr.promeStatMgr = newPromeStatMgr(name, func(defaultLabels, labels []string) prometheus.Collector {
		return newGagueVec(name, defaultLabels, labels)
	})
	return mgr
}

func NewHistogram(name string, buckets []float64) *PromeHistogramStatMgr {
	mgr := new(PromeHistogramStatMgr)
	mgr.promeStatMgr = newPromeStatMgr(name, func(defaultLabels, labels []string) prometheus.Collector {
		return newHistogramVec(name, defaultLabels, labels, buckets)
	})
	return mgr
}

type PromeCounterStatMgr struct {
	*promeStatMgr
}

func (c *PromeCounterStatMgr) InitLabels(labels []string) *PromeCounterStatMgr {
	return c.InitDefaultLabels(nil, labels)
}

func (c *PromeCounterStatMgr) InitDefaultLabels(defaultLabels map[string]string, labels []string) *PromeCounterStatMgr {
	c.promeStatMgr.withDefaultLabels(defaultLabels, labels)
	return c
}

func (c *PromeCounterStatMgr) LabelValues(labels ...string) prometheus.Counter {
	return c.promeStatMgr.getCounterWithLabels(labels...)
}

type PromeGaugeStatMgr struct {
	*promeStatMgr
}

func (c *PromeGaugeStatMgr) InitLabels(labels []string) *PromeGaugeStatMgr {
	return c.InitDefaultLabels(nil, labels)
}

func (c *PromeGaugeStatMgr) InitDefaultLabels(defaultLabels map[string]string, labels []string) *PromeGaugeStatMgr {
	c.promeStatMgr.withDefaultLabels(defaultLabels, labels)
	return c
}

func (c *PromeGaugeStatMgr) LabelValues(labels ...string) prometheus.Gauge {
	return c.promeStatMgr.getGaugeWithLabels(labels...)
}

type PromeHistogramStatMgr struct {
	*promeStatMgr
}

func (c *PromeHistogramStatMgr) InitLabels(labels []string) *PromeHistogramStatMgr {
	return c.InitDefaultLabels(nil, labels)
}

func (c *PromeHistogramStatMgr) InitDefaultLabels(defaultLabels map[string]string, labels []string) *PromeHistogramStatMgr {
	c.promeStatMgr.withDefaultLabels(defaultLabels, labels)
	return c
}

func (c *PromeHistogramStatMgr) LabelValues(labels ...string) prometheus.Observer {
	return c.promeStatMgr.getHistogramWithLabels(labels...)
}

type promeStatMgr struct {
	collector          prometheus.Collector
	name               string
	newCollectorFun    func(defaultLabels, labels []string) prometheus.Collector
	defaultLabelsValue []string
	initCounter        int32
}

func (mgr *promeStatMgr) withDefaultLabels(defaultLabels map[string]string, labels []string) {
	if atomic.AddInt32(&mgr.initCounter, 1) > 1 {
		panic(fmt.Errorf("promethus vec labels must init once"))
	}

	defaultLabelKeys := make([]string, 0)
	defaultLabelValues := make([]string, 0)
	for k, v := range defaultLabels {
		defaultLabelKeys = append(defaultLabelKeys, k)
		defaultLabelValues = append(defaultLabelValues, v)
	}

	mgr.defaultLabelsValue = defaultLabelValues
	mgr.collector = mgr.newCollectorFun(defaultLabelKeys, labels)
	prometheus.MustRegister(mgr.collector)
	return
}

func (mgr *promeStatMgr) joinLabels(labels ...string) []string {
	newLabels := make([]string, 0, len(mgr.defaultLabelsValue)+len(labels))
	newLabels = append(newLabels, mgr.defaultLabelsValue...)
	newLabels = append(newLabels, labels...)
	return newLabels
}

func (mgr *promeStatMgr) getCounterWithLabels(labels ...string) prometheus.Counter {
	newLabels := mgr.joinLabels(labels...)
	return mgr.collector.(*prometheus.CounterVec).WithLabelValues(newLabels...)
}

func (mgr *promeStatMgr) getGaugeWithLabels(labels ...string) prometheus.Gauge {
	newLabels := mgr.joinLabels(labels...)
	return mgr.collector.(*prometheus.GaugeVec).WithLabelValues(newLabels...)
}

func (mgr *promeStatMgr) getHistogramWithLabels(labels ...string) prometheus.Observer {
	newLabels := mgr.joinLabels(labels...)
	return mgr.collector.(*prometheus.HistogramVec).WithLabelValues(newLabels...)
}

func newPromeStatMgr(name string,
	newCollectorFun func(defaultLabelsKey []string, labels []string) prometheus.Collector) *promeStatMgr {
	mgr := new(promeStatMgr)
	mgr.name = name
	mgr.newCollectorFun = newCollectorFun
	return mgr
}

func newCounterVec(name string, defaultLabels []string, dynamicLabels []string) prometheus.Collector {
	vec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: name,
		},
		append(defaultLabels, dynamicLabels...),
	)
	return vec
}

func newGagueVec(name string, defaultLabels []string, dynamicLabels []string) prometheus.Collector {
	vec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
			Help: name,
		},
		append(defaultLabels, dynamicLabels...),
	)
	return vec
}

func newHistogramVec(name string, defaultLabels []string, dynamicLabels []string, buckets []float64) prometheus.Collector {
	vec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    name,
			Help:    name,
			Buckets: buckets,
		},
		append(defaultLabels, dynamicLabels...),
	)
	return vec
}

func NewEngine(addr string, enablePprof bool) *jweb.Engine {
	engine := jweb.NewEngine(addr, func() jweb.Context {
		return new(Context)
	})

	if enablePprof {
		ginPprof.Register(engine.GetGinEngine())
	}

	ginF := gin.WrapH(promhttp.Handler())
	engine.Get("/metrics", "metrics", func(c *Context) {
		ginF(c.GetGinContext())
	})

	return engine
}

type Context struct {
	ginCtx *gin.Context
}

func (c *Context) SetGinContext(ctx *gin.Context) {
	c.ginCtx = ctx
}
func (c *Context) GetGinContext() *gin.Context {
	return c.ginCtx
}
func (c *Context) ResponseParseParamsFieldFail(path string, field string, value string, err error) {

}

func RouteEngine(engine *gin.Engine, enablePprof bool) {
	if enablePprof {
		ginPprof.Register(engine)
	}
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
