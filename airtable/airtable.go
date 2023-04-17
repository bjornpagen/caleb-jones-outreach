package airtable

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"go.uber.org/ratelimit"
)

type Option func(option *options) error

type options struct {
	host       string
	rateLimit  *ratelimit.Limiter
	httpClient *http.Client
}

func WithHost(host string) Option {
	return func(option *options) error {
		// Check if host is valid.
		_, err := http.NewRequest("GET", fmt.Sprintf("https://%s", host), nil)
		if err != nil {
			return fmt.Errorf("invalid host: %w", err)
		}

		option.host = host
		return nil
	}
}

func WithRateLimit(rl ratelimit.Limiter) Option {
	return func(option *options) error {
		option.rateLimit = &rl
		return nil
	}
}

func WithHttpClient(hc http.Client) Option {
	return func(option *options) error {
		option.httpClient = &hc
		return nil
	}
}

type Client struct {
	apiKey  string
	options *options
}

func New(apiKey string, opts ...Option) (*Client, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, fmt.Errorf("bad option: %w", err)
		}
	}

	if o.host == "" {
		o.host = "api.airtable.com/v0"
	}

	if o.rateLimit == nil {
		o.rateLimit = new(ratelimit.Limiter)
		*o.rateLimit = ratelimit.New(5, ratelimit.Per(time.Second))
	}

	if o.httpClient == nil {
		o.httpClient = http.DefaultClient
	}

	return &Client{
		apiKey:  apiKey,
		options: o,
	}, nil
}

type param struct {
	key   string
	value string
}

func (c *Client) buildUrl(p []string) string {
	return fmt.Sprintf("https://%s/%s", c.options.host, path.Join(p...))
}

func (c *Client) buildUrlWithParameters(path []string, params []param) string {
	url := c.buildUrl(path)
	for i, p := range params {
		separator := "&"
		if i == 0 {
			separator = "?"
		}
		url = fmt.Sprintf("%s%s%s=%s", url, separator, p.key, p.value)
	}
	return url
}

func (c *Client) do(req *http.Request) (data []byte, err error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	(*c.options.rateLimit).Take()
	res, err := c.options.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status code %d", res.StatusCode)
	}

	data, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (c *Client) get(path []string, params []param) (data []byte, err error) {
	url := c.buildUrlWithParameters(path, params)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req)
}

func (c *Client) post(path []string, body any) (data []byte, err error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	url := c.buildUrl(path)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req)
}

func (c *Client) patch(path []string, body any) (data []byte, err error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	url := c.buildUrl(path)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req)
}

func (c *Client) delete(path []string) (data []byte, err error) {
	url := c.buildUrl(path)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req)
}

// Airtable Types

type ShortText string

type User struct {
	Id    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type SingleSelect string

type Number float64

type URL string

type Email string

type Phone string

type Record[T any] struct {
	Id          string    `json:"id,omitempty"`
	CreatedTime time.Time `json:"createdTime,omitempty"`
	Fields      *T        `json:"fields"`
}

type Page[T any] struct {
	Records []Record[T] `json:"records"`
	Offset  string      `json:"offset"`
}

type Table[T any] struct {
	c       *Client
	baseId  string
	tableId string
}

func NewTable[T any](c *Client, baseId, tableId string) *Table[T] {
	return &Table[T]{
		c:       c,
		baseId:  baseId,
		tableId: tableId,
	}
}

func (l *Table[T]) List() ([]Record[T], error) {
	var records []Record[T]

	offset := ""
	for {
		page, err := l.list(offset)
		if err != nil {
			return records, err
		}

		records = append(records, page.Records...)
		if page.Offset == "" {
			break
		}

		offset = page.Offset
	}

	return records, nil
}

func (l *Table[T]) list(offset string) (*Page[T], error) {
	params := []param{}
	if offset != "" {
		params = append(params, param{key: "offset", value: offset})
	}

	data, err := l.c.get([]string{l.baseId, l.tableId}, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get records: %w", err)
	}

	page := &Page[T]{}
	err = json.Unmarshal(data, page)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return page, nil
}

func (l *Table[T]) Retrieve(recordId string) (*Record[T], error) {
	data, err := l.c.get([]string{l.baseId, l.tableId, recordId}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	record := &Record[T]{}
	err = json.Unmarshal(data, record)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return record, nil
}

func (l *Table[T]) Create(record Record[T]) (*Record[T], error) {
	data, err := l.c.post([]string{l.baseId, l.tableId}, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	newRecord := &Record[T]{}
	err = json.Unmarshal(data, newRecord)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return newRecord, nil
}

func (l *Table[T]) Update(records []Record[T]) ([]Record[T], error) {
	// Split records into chunks of 10
	var chunks [][]Record[T]
	for i := 0; i < len(records); i += 10 {
		end := i + 10
		if end > len(records) {
			end = len(records)
		}
		chunks = append(chunks, records[i:end])
	}

	var updatedRecords []Record[T]
	for _, chunk := range chunks {
		page, err := l.update(chunk)
		if err != nil {
			return updatedRecords, err
		}

		updatedRecords = append(updatedRecords, page.Records...)
	}

	return updatedRecords, nil
}

func (l *Table[T]) update(records []Record[T]) (*Page[T], error) {
	data, err := l.c.patch([]string{l.baseId, l.tableId}, records)
	if err != nil {
		return nil, fmt.Errorf("failed to update records: %w", err)
	}

	page := &Page[T]{}
	err = json.Unmarshal(data, page)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return page, nil
}

func (l *Table[T]) Delete(recordId string) error {
	_, err := l.c.delete([]string{l.baseId, l.tableId, recordId})
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}
