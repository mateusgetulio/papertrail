package brave

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/mateusgetulio/papertrail/internal/search"
)

const baseURL = "https://api.search.brave.com/res/v1/web/search"

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Name() string { return "brave" }

func (c *Client) Search(ctx context.Context, query string, count int) ([]search.Result, error) {
	if count <= 0 || count > 20 {
		count = 10
	}
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", count))
	params.Set("search_lang", "en")
	params.Set("result_filter", "web")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("brave: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave: status %d", resp.StatusCode)
	}

	var body braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("brave: decode: %w", err)
	}

	results := make([]search.Result, 0, len(body.Web.Results))
	for _, r := range body.Web.Results {
		parsed, err := url.Parse(r.URL)
		if err != nil {
			continue
		}
		results = append(results, search.Result{
			URL:          r.URL,
			Title:        r.Title,
			Description:  r.Description,
			SourceDomain: parsed.Hostname(),
		})
	}
	return results, nil
}

type braveResponse struct {
	Web struct {
		Results []struct {
			URL         string `json:"url"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}
