package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/palantir/go-githubapp/githubapp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type statusCodeResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusCodeResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

type LoggerFunc func(msg string, m map[string]interface{})

func loggingContextMiddleware(logger zerolog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := logger.With()

		l := ctx.Logger()

		r = r.WithContext(l.WithContext(r.Context()))

		h.ServeHTTP(w, r)
	})
}

func loggerMiddleware(logger LoggerFunc, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusWriter := &statusCodeResponseWriter{w, http.StatusOK}
		start := time.Now()

		h.ServeHTTP(statusWriter, r)

		requestDuration := time.Since(start).Milliseconds()
		statusCode := statusWriter.statusCode

		logMessage := fmt.Sprintf("[%d] %s %s", statusCode, r.Method, r.URL.String())
		logMap := map[string]interface{}{
			"responseTime": fmt.Sprintf("%d", requestDuration),
		}

		logger(logMessage, logMap)
	})
}

func inboundMetricsMiddleware(promRegisterer prometheus.Registerer, h http.Handler) http.Handler {

	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inbound_http_status_code_total",
			Help: "Counter of HTTP status codes of inbound HTTP requests",
		},
		[]string{"status_code", "method"},
	)

	millisBuckets := []float64{5, 10, 50, 100, 250, 500, 1000, 1500, 2000, 3000, 4000, 5000, 10000}
	httpRequestsDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "inbound_http_duration_milliseconds",
			Help:    "Histogram of response latency (in milliseconds) of inbound requests",
			Buckets: millisBuckets,
		})

	promRegisterer.MustRegister(httpRequests)
	promRegisterer.MustRegister(httpRequestsDuration)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusWriter := &statusCodeResponseWriter{w, http.StatusOK}
		start := time.Now()

		h.ServeHTTP(statusWriter, r)

		requestDuration := time.Since(start).Milliseconds()
		statusCode := statusWriter.statusCode

		httpRequests.WithLabelValues(strconv.Itoa(statusCode), r.Method).Inc()
		httpRequestsDuration.Observe(float64(requestDuration))
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func clientMetricsMiddleware(promRegisterer prometheus.Registerer, requestDestination string) func(http.RoundTripper) http.RoundTripper {

	metricHTTPRequestsStatus := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "outbound_http_status_code_total",
			Help:        "Counter of HTTP status codes of outbound HTTP requests",
			ConstLabels: prometheus.Labels{"destination": requestDestination},
		},
		[]string{"status_code"},
	)

	promRegisterer.MustRegister(metricHTTPRequestsStatus)

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(r *http.Request) (*http.Response, error) {

			res, err := next.RoundTrip(r)
			if err != nil {
				return res, err
			}
			if res == nil {
				return res, err
			}

			metricHTTPRequestsStatus.WithLabelValues(strconv.Itoa(res.StatusCode)).Inc()

			return res, err
		})
	}
}

func githubMetricsMiddleware(promRegisterer prometheus.Registerer) githubapp.ClientMiddleware {

	// Headers from https://developer.github.com/v3/#rate-limiting
	metricGithubRatelimitLimit := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_limit_info",
			Help: "Gauge of Github X-RateLimit-Limit header. The maximum number of requests you're permitted to make per hour",
		},
	)
	metricGithubRatelimitRemaining := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_remaining_info",
			Help: "Gauge of Github X-RateLimit-Remaining header. The number of requests remaining in the current rate limit window",
		},
	)
	metricGithubRatelimitReset := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_reset_UNIX_time",
			Help: "Gauge of Github X-RateLimit-Reset header. The time at which the current rate limit window resets in UTC epoch seconds",
		},
	)
	metricGithubRatelimitErrors := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "github_ratelimit_errors_total",
			Help: "Counter of errors retrieving the X-RateLimit-?? headers",
		},
	)

	promRegisterer.MustRegister(metricGithubRatelimitLimit)
	promRegisterer.MustRegister(metricGithubRatelimitRemaining)
	promRegisterer.MustRegister(metricGithubRatelimitReset)
	promRegisterer.MustRegister(metricGithubRatelimitErrors)

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(r *http.Request) (*http.Response, error) {

			res, err := next.RoundTrip(r)
			if err != nil {
				return res, err
			}
			if res == nil {
				return res, err
			}

			githubRatelimitLimit, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Limit"), 64)
			if err != nil {
				metricGithubRatelimitErrors.Inc()
			} else {
				metricGithubRatelimitLimit.Set(githubRatelimitLimit)
			}

			githubRatelimitRemaining, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Remaining"), 64)
			if err != nil {
				metricGithubRatelimitErrors.Inc()
			} else {
				metricGithubRatelimitRemaining.Set(githubRatelimitRemaining)
			}

			githubRatelimitReset, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Reset"), 64)
			if err != nil {
				metricGithubRatelimitErrors.Inc()
			} else {
				metricGithubRatelimitReset.Set(githubRatelimitReset)
			}

			return res, err
		})
	}
}
