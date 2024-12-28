package option

import "go.uber.org/ratelimit"

type GoogleSheetClientOption interface {
	Apply(*GoogleSheetClient)
}

type googleSheetClientOptionFunc func(*GoogleSheetClient)

func (f googleSheetClientOptionFunc) Apply(o *GoogleSheetClient) {
	f(o)
}

func WithGoogleSheetClientCredentialJSON(credentialJSON []byte) GoogleSheetClientOption {
	return googleSheetClientOptionFunc(func(o *GoogleSheetClient) {
		o.CredentialJSON = credentialJSON
	})
}

func WithGoogleSheetClientRateLimiter(rateLimiter ratelimit.Limiter) GoogleSheetClientOption {
	return googleSheetClientOptionFunc(func(o *GoogleSheetClient) {
		o.RateLimiter = rateLimiter
	})
}

type GoogleSheetClient struct {
	CredentialJSON []byte
	RateLimiter    ratelimit.Limiter
}
