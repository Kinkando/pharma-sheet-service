package option

import (
	"net/http"

	"go.uber.org/ratelimit"
)

type HTTPInterceptorRateLimiterOption interface {
	Apply(*HTTPInterceptorRateLimiter)
}

type httpInterceptorRateLimiterOptionFunc func(*HTTPInterceptorRateLimiter)

func (hitrof httpInterceptorRateLimiterOptionFunc) Apply(hitro *HTTPInterceptorRateLimiter) {
	hitrof(hitro)
}

func WithHTTPInterceptorRateLimiterTransport(transport http.RoundTripper) HTTPInterceptorRateLimiterOption {
	return httpInterceptorRateLimiterOptionFunc(func(hitro *HTTPInterceptorRateLimiter) {
		hitro.Transport = transport
	})
}

func WithHTTPInterceptorRateLimiterRateLimiter(rateLimiter ratelimit.Limiter) HTTPInterceptorRateLimiterOption {
	return httpInterceptorRateLimiterOptionFunc(func(hitro *HTTPInterceptorRateLimiter) {
		hitro.RateLimiter = rateLimiter
	})
}

type HTTPInterceptorRateLimiter struct {
	Transport   http.RoundTripper
	RateLimiter ratelimit.Limiter
}
