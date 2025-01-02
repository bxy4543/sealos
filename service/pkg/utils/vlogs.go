package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	BaseURL    string
	Username   string
	Password   string
	httpClient *http.Client
}

type HigressLogEntry struct {
	Count     string `json:"count"`
	Authority string `json:"authority"`
	RouteName string `json:"route_name"`
}

func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:    baseURL,
		Username:   username,
		Password:   password,
		httpClient: &http.Client{},
	}
}

func (c *Client) Request(endpoint string, params map[string]string) ([]byte, error) {
	data := url.Values{}
	for key, value := range params {
		data.Set(key, value)
	}

	req, err := http.NewRequest("POST", c.BaseURL+endpoint, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.Username != "" && c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// QueryLogs 查询日志
func (c *Client) QueryLogs(query string, limit int, start, end time.Time) ([]byte, error) {
	params := map[string]string{
		"query": query,
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}
	return c.Request("/select/logsql/query", params)
}

// TailLogs 实时获取日志
func (c *Client) TailLogs(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/tail", params)
}

// QueryHits 查询日志命中统计
func (c *Client) QueryHits(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/hits", params)
}

// QueryFacets 查询字段的最频繁值
func (c *Client) QueryFacets(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/facets", params)
}

// StatsQuery 查询日志统计
func (c *Client) StatsQuery(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/stats_query", params)
}

// StatsQueryRange 查询日志统计范围
func (c *Client) StatsQueryRange(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/stats_query_range", params)
}

// QueryStreamIDs 查询日志流的 _stream_id 值
func (c *Client) QueryStreamIDs(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/stream_ids", params)
}

// QueryStreams 查询日志流
func (c *Client) QueryStreams(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/streams", params)
}

// QueryStreamFieldNames 查询日志流字段名
func (c *Client) QueryStreamFieldNames(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/stream_field_names", params)
}

// QueryStreamFieldValues 查询日志流字段值
func (c *Client) QueryStreamFieldValues(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/stream_field_values", params)
}

// QueryFieldNames 查询日志字段名
func (c *Client) QueryFieldNames(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/field_names", params)
}

// QueryFieldValues 查询日志字段值
func (c *Client) QueryFieldValues(params map[string]string) ([]byte, error) {
	return c.Request("/select/logsql/field_values", params)
}
