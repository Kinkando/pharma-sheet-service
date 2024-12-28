package httpinterceptor

import (
	"net/http"
	"time"

	"github.com/kinkando/pharma-sheet-service/pkg/option"
	"go.uber.org/ratelimit"
)

type RateLimiterTransport struct {
	Transport   http.RoundTripper
	RateLimiter ratelimit.Limiter
}

func NewRateLimiterTransport(opts ...option.HTTPInterceptorRateLimiterOption) *RateLimiterTransport {
	hi := &option.HTTPInterceptorRateLimiter{
		Transport:   http.DefaultTransport,
		RateLimiter: ratelimit.New(100, ratelimit.Per(time.Second)),
	}
	for _, opt := range opts {
		opt.Apply(hi)
	}

	return &RateLimiterTransport{
		Transport:   hi.Transport,
		RateLimiter: hi.RateLimiter,
	}
}

func (rt *RateLimiterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.RateLimiter.Take()
	return rt.Transport.RoundTrip(req)
}
