package main

import (
	"context"
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
func InitializeResponder(ctx context.Context) context.Context {
	var responder func(http.ResponseWriter, *http.Request)
	return context.WithValue(ctx, responderKey{}, &responder)
}

type responderKey struct{}
type LoggerFunc func(msg string, m map[string]interface{})

func addContextMiddleware(logger zerolog.Logger, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//eventType := r.Header.Get("X-GitHub-Event")
		//deliveryID := r.Header.Get("X-GitHub-Delivery")

		ctx := logger.With()
		//ctx = ctx.Str("github_event_type", eventType)
		//ctx = ctx.Str("github_delivery_id", deliveryID)

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

		// request duration in miliseconds
		duration := time.Since(start).Nanoseconds() / 1e6
		statusCode := statusWriter.statusCode

		logMessage := fmt.Sprintf("[%d] %s %s", statusCode, r.Method, r.URL.String())
		logMap := map[string]interface{}{
			"responseTime": fmt.Sprintf("%d", duration),
		}

		logger(logMessage, logMap)
	})
}

func metricsMiddleware(promRegisterer prometheus.Registerer, h http.Handler) http.Handler {

	httpRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "inbound_http_status_code",
			Help: "Counter of HTTP status codes of inbound HTTP requests",
		},
		[]string{"status_code", "method"},
	)
	promRegisterer.MustRegister(httpRequests)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusWriter := &statusCodeResponseWriter{w, http.StatusOK}
		//start := time.Now()
		h.ServeHTTP(statusWriter, r)

		// request duration in miliseconds
		//duration := time.Since(start).Nanoseconds() / 1e6
		statusCode := statusWriter.statusCode

		httpRequests.WithLabelValues(strconv.Itoa(statusCode), r.Method).Inc()

	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)
type ClientMiddleware func(http.RoundTripper) http.RoundTripper

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func clientMetricsMiddleware(promRegisterer prometheus.Registerer) githubapp.ClientMiddleware {

	metricHTTPRequestsStatus := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "outbound_http_status_code_total",
			Help: "Counter of HTTP status codes of outbound HTTP requests",
		},
		[]string{"status_code"},
	)

	// Headers from https://developer.github.com/v3/#rate-limiting
	metricGithubRatelimitLimit := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_limit_info",
			Help: "Gauge of Github X-RateLimit-Limit header",
		},
	)
	metricGithubRatelimitRemaining := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_remaining_info",
			Help: "Gauge of Github X-RateLimit-Remaining header",
		},
	)
	metricGithubRatelimitReset := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "github_ratelimit_reset_info",
			Help: "Gauge of Github X-RateLimit-Reset header",
		},
	)

	promRegisterer.MustRegister(metricHTTPRequestsStatus)
	promRegisterer.MustRegister(metricGithubRatelimitLimit)
	promRegisterer.MustRegister(metricGithubRatelimitRemaining)
	promRegisterer.MustRegister(metricGithubRatelimitReset)

	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(r *http.Request) (*http.Response, error) {

			/* installationID, ok := r.Context().Value("installationID").(int64)
			if !ok {
				installationID = 0
			} */

			res, err := next.RoundTrip(r)
			if err != nil {
				// todo
			}

			if res != nil {

				githubRatelimitLimit, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Limit"), 64)
				if err != nil {
					// todo
				}

				githubRatelimitRemaining, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Remaining"), 64)
				if err != nil {
					// todo
				}

				githubRatelimitReset, err := strconv.ParseFloat(res.Header.Get("X-RateLimit-Reset"), 64)
				if err != nil {
					// todo
				}

				metricHTTPRequestsStatus.WithLabelValues(strconv.Itoa(res.StatusCode)).Inc()
				metricGithubRatelimitLimit.Set(githubRatelimitLimit)
				metricGithubRatelimitRemaining.Set(githubRatelimitRemaining)
				metricGithubRatelimitReset.Set(githubRatelimitReset)
			}

			return res, err
		})
	}
}
