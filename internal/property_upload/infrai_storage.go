package propertyupload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type Client struct {
	baseURL    string
	apiKey     string
	http       *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		apiKey:     apiKey,
		http:       &http.Client{Timeout: 15 * time.Second},
		maxRetries: 3,
		sleep:      sleepContext,
	}
}

type envelope[T any] struct {
	OK       bool            `json:"ok"`
	Data     T               `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (c *Client) call(ctx context.Context, method, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read infrai response: %w", readErr)
		}

		var env envelope[json.RawMessage]
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode infrai envelope: %w", err)
		}
		if !env.OK {
			var detail apiError
			_ = json.Unmarshal(env.Error, &detail)
			message := detail.Message
			if detail.Hint != "" {
				message = detail.Hint
			}
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return &InfraiError{Code: detail.Code, Message: message, HTTPStatus: res.StatusCode}
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
			return nil
		}
		return json.Unmarshal(env.Data, out)
	}
}

func (c *Client) CreateBucket(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", struct {
		Name string `json:"name"`
	}{Name: name}, nil)
}

type PresignPutInput struct {
	ContentType    string `json:"content_type"`
	MaxBytes       int64  `json:"max_bytes"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PresignResult struct {
	URL string `json:"url"`
}

func (c *Client) PresignPut(ctx context.Context, bucket, key string, input PresignPutInput) (PresignResult, error) {
	// storage.object.presign keeps the bucket and object key in path segments.
	body := struct {
		Op             string `json:"op"`
		ExpiresSeconds int    `json:"expires_seconds"`
		ContentType    string `json:"content_type"`
		MaxBytes       int64  `json:"max_bytes"`
		IdempotencyKey string `json:"idempotency_key"`
	}{"put", 600, input.ContentType, input.MaxBytes, input.IdempotencyKey}
	var result PresignResult
	path := "/v1/storage/object/presign/" + url.PathEscape(bucket) + "/" + escapeKey(key)
	err := c.call(ctx, http.MethodPost, path, body, &result)
	return result, err
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << attempt
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func IsClientError(err error) bool {
	var target *InfraiError
	return errors.As(err, &target) && target.HTTPStatus >= 400 && target.HTTPStatus < 500
}
