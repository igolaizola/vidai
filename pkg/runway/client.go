package runway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/golang-jwt/jwt"
	"github.com/igopr/vidai/pkg/fhttp"
	"github.com/igopr/vidai/pkg/ratelimit"
)

type Client struct {
	client     fhttp.Client
	debug      bool
	ratelimit  ratelimit.Lock
	token      string
	expiration time.Time
	teamID     int
}

type Config struct {
	Token string
	Wait  time.Duration
	Debug bool
	Proxy string
}

func New(cfg *Config) (*Client, error) {
	wait := cfg.Wait
	if wait == 0 {
		wait = 1 * time.Second
	}
	// Parse the JWT
	parser := jwt.Parser{}
	t, _, err := parser.ParseUnverified(cfg.Token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("runway: couldn't parse token: %w", err)
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("runway: couldn't parse claims: %w", err)
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("runway: couldn't parse expiration: %w", err)
	}
	expiration := time.Unix(int64(exp), 0)
	if expiration.Before(time.Now()) {
		return nil, fmt.Errorf("runway: token expired")
	}
	client := fhttp.NewClient(2*time.Minute, true, cfg.Proxy)
	return &Client{
		client:     client,
		ratelimit:  ratelimit.New(wait),
		debug:      cfg.Debug,
		token:      cfg.Token,
		expiration: expiration,
	}, nil
}

func (c *Client) log(format string, args ...interface{}) {
	if c.debug {
		format += "\n"
		log.Printf(format, args...)
	}
}

var backoff = []time.Duration{
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) ([]byte, error) {
	if time.Now().After(c.expiration) {
		return nil, fmt.Errorf("runway: token expired")
	}
	maxAttempts := 3
	attempts := 0
	var err error
	for {
		if err != nil {
			log.Println("retrying...", err)
		}
		var b []byte
		b, err = c.doAttempt(ctx, method, path, in, out)
		if err == nil {
			return b, nil
		}
		// Increase attempts and check if we should stop
		attempts++
		if attempts >= maxAttempts {
			return nil, err
		}
		// If the error is temporary retry
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			continue
		}
		// Check status code
		var errStatus errStatusCode
		if errors.As(err, &errStatus) {
			switch int(errStatus) {
			// These errors are retriable but we should wait before retry
			case http.StatusBadGateway, http.StatusGatewayTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, 520, 522:
			default:
				return nil, err
			}

			idx := attempts - 1
			if idx >= len(backoff) {
				idx = len(backoff) - 1
			}
			wait := backoff[idx]
			c.log("server seems to be down, waiting %s before retrying\n", wait)
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-t.C:
			}
			continue
		}
		return nil, err
	}
}

type errStatusCode int

func (e errStatusCode) Error() string {
	return fmt.Sprintf("%d", e)
}

func (c *Client) doAttempt(ctx context.Context, method, path string, in, out any) ([]byte, error) {
	var body []byte
	var reqBody io.Reader
	contentType := "application/json"
	if f, ok := in.(*uploadFile); ok {
		body = f.data
		ext := f.extension
		if ext == "jpg" {
			ext = "jpeg"
		}
		contentType = fmt.Sprintf("image/%s", ext)
		reqBody = bytes.NewReader(body)
	} else if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("runway: couldn't marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(body)
	}
	logBody := string(body)
	/*if len(logBody) > 100 {
		logBody = logBody[:100] + "..."
	}*/
	c.log("runway: do %s %s %s", method, path, logBody)

	// Check if path is absolute
	u := fmt.Sprintf("https://api.runwayml.com/v1/%s", path)
	var uploadLen int
	if strings.HasPrefix(path, "http") {
		u = path
		uploadLen = len(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("runway: couldn't create request: %w", err)
	}
	c.addHeaders(req, path, contentType, uploadLen)

	unlock := c.ratelimit.Lock(ctx)
	defer unlock()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("runway: couldn't %s %s: %w", method, u, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("runway: couldn't read response body: %w", err)
	}
	c.log("runway: response %s %s %d %s", method, path, resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK {
		errMessage := string(respBody)
		if len(errMessage) > 100 {
			errMessage = errMessage[:100] + "..."
		}
		_ = os.WriteFile(fmt.Sprintf("logs/debug_%s.json", time.Now().Format("20060102_150405")), respBody, 0644)
		return nil, fmt.Errorf("runway: %s %s returned (%s): %w", method, u, errMessage, errStatusCode(resp.StatusCode))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			// Write response body to file for debugging.
			_ = os.WriteFile(fmt.Sprintf("logs/debug_%s.json", time.Now().Format("20060102_150405")), respBody, 0644)
			return nil, fmt.Errorf("runway: couldn't unmarshal response body (%T): %w", out, err)
		}
	}
	return respBody, nil
}

func (c *Client) addHeaders(req *http.Request, path, contentType string, uploadLen int) {
	switch {
	case uploadLen > 0:
		req.Header.Set("accept", "*/*")
		req.Header.Set("accept-language", "en-US,en;q=0.9")
		req.Header.Set("content-length", fmt.Sprintf("%d", uploadLen))
		req.Header.Set("content-type", contentType)
		req.Header.Set("connection", "keep-alive")
		req.Header.Set("origin", "https://app.runwayml.com")
		req.Header.Set("priority", "u=1, i")
		req.Header.Set("referer", "https://app.runwayml.com/")
		req.Header.Set("sec-ch-ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Set("sec-fetch-dest", "empty")
		req.Header.Set("sec-fetch-mode", "cors")
		req.Header.Set("sec-fetch-site", "cross-site")
		req.Header.Set("user-agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36`)
	case !strings.HasPrefix(path, "http"):
		req.Header.Set("accept", "application/json")
		req.Header.Set("accept-language", "en-US,en;q=0.9")
		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", c.token))
		req.Header.Set("content-type", contentType)
		req.Header.Set("origin", "https://app.runwayml.com")
		req.Header.Set("priority", "u=1, i")
		req.Header.Set("referer", "https://app.runwayml.com/")
		req.Header.Set("sec-ch-ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Set("sec-fetch-dest", "empty")
		req.Header.Set("sec-fetch-mode", "cors")
		req.Header.Set("sec-fetch-site", "same-site")
		req.Header.Set("user-agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36`)
		// TODO: Add sentry trace if needed.
		// req.Header.Set("Sentry-Trace", "TODO")
	default:
		req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("accept-language", "en-US,en;q=0.9")
		req.Header.Set("origin", fmt.Sprintf("%s://%s", req.URL.Scheme, req.URL.Host))
		req.Header.Set("priority", "u=1, i")
		req.Header.Set("referer", "https://app.runwayml.com/")
		req.Header.Set("sec-ch-ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", "\"Windows\"")
		req.Header.Set("sec-fetch-dest", "empty")
		req.Header.Set("sec-fetch-mode", "cors")
		req.Header.Set("sec-fetch-site", "same-site")
		req.Header.Set("user-agent", `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36`)
	}
}
