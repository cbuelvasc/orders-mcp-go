// Package observability exposes Prometheus metrics with the SAME names as the Java version,
// so the same Grafana dashboard and k6 thresholds work for both (fair comparison).
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry     *prometheus.Registry
	toolDuration *prometheus.HistogramVec
	toolErrors   *prometheus.CounterVec
	httpRequests *prometheus.HistogramVec
}

func New() *Metrics {
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5} // includes the SLO edges
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		toolDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "mcp_tool_duration_seconds", Help: "Duration of MCP tool calls.", Buckets: buckets,
		}, []string{"tool"}),
		toolErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mcp_tool_errors_total", Help: "Failed MCP tool calls.",
		}, []string{"tool", "exception"}),
		httpRequests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "http_server_requests_seconds", Help: "HTTP requests.", Buckets: buckets,
		}, []string{"method", "uri", "status"}),
	}
	m.registry.MustRegister(m.toolDuration, m.toolErrors, m.httpRequests,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// ObserveTool implements the MCP adapter's Metrics port.
func (m *Metrics) ObserveTool(tool string, elapsed time.Duration, errKind string) {
	m.toolDuration.WithLabelValues(tool).Observe(elapsed.Seconds())
	if errKind != "" {
		m.toolErrors.WithLabelValues(tool, errKind).Inc()
	}
}

// Handler serves /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Instrument records http_server_requests_seconds{method,uri,status} for one route.
func (m *Metrics) Instrument(uri string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		m.httpRequests.WithLabelValues(r.Method, uri, strconv.Itoa(sw.status)).Observe(time.Since(start).Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush keeps Server-Sent Events (Streamable HTTP) working through the wrapper.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
