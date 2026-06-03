package compliance

import (
	"context"
	"net/url"
)

// Decision is the result of the allowed-fetch gate (docs/03 §2).
type Decision struct {
	Allowed  bool
	Reason   string
	RobotsOK bool
	Gated    bool
}

// Gate checks whether a URL is safe to fetch under compliance rules.
// Implement compliance/gate_impl.go for the full robots.txt + policy + rate-limit logic.
type Gate interface {
	Check(ctx context.Context, u *url.URL) (Decision, error)
}

// AlwaysDeny is a safe default Gate used until the real implementation is wired.
type AlwaysDeny struct{}

func (AlwaysDeny) Check(_ context.Context, u *url.URL) (Decision, error) {
	return Decision{
		Allowed:  false,
		Reason:   "compliance gate not yet implemented — all fetches denied by default",
		RobotsOK: false,
		Gated:    false,
	}, nil
}
