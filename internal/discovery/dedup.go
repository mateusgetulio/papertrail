package discovery

import (
	"crypto/sha256"
	"net/url"
	"sort"
	"strings"
)

// Canonical returns a normalised, lowercased URL string suitable for deduplication.
// It: lowercases scheme+host, removes default ports, sorts query params, strips fragments.
func Canonical(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	u.Scheme = strings.ToLower(u.Scheme)

	// remove default ports
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		u.Host = host
	}

	// normalise path: collapse double slashes, lowercase
	u.Path = strings.ToLower(strings.ReplaceAll(u.Path, "//", "/"))
	if u.Path == "" {
		u.Path = "/"
	}

	// sort query params for stable fingerprint
	if u.RawQuery != "" {
		params := u.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			for _, v := range params[k] {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		u.RawQuery = strings.Join(parts, "&")
	}

	return u.String(), nil
}

// URLHash returns a SHA-256 fingerprint of the canonical URL.
func URLHash(canonical string) []byte {
	h := sha256.Sum256([]byte(canonical))
	return h[:]
}

// Deduplicator tracks seen URL hashes in memory (backed by DB in production).
type Deduplicator struct {
	seen map[string]struct{}
}

func NewDeduplicator() *Deduplicator {
	return &Deduplicator{seen: make(map[string]struct{})}
}

// IsDuplicate returns true if the canonical URL has been seen before, and records it.
func (d *Deduplicator) IsDuplicate(canonical string) bool {
	if _, ok := d.seen[canonical]; ok {
		return true
	}
	d.seen[canonical] = struct{}{}
	return false
}
