package fhttp

import (
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Client interface {
	tlsclient.HttpClient
}

type client struct {
	tlsclient.HttpClient
}

func NewClient(timeout time.Duration, useJar bool, proxy string) Client {
	jar := tlsclient.NewCookieJar()
	secs := int(timeout.Seconds())
	if secs <= 0 {
		secs = 30
	}
	options := []tlsclient.HttpClientOption{
		tlsclient.WithTimeoutSeconds(secs),
		tlsclient.WithClientProfile(profiles.Chrome_120),
		tlsclient.WithNotFollowRedirects(),
	}
	if useJar {
		options = append(options, tlsclient.WithCookieJar(jar))
	}
	if proxy != "" {
		options = append(options, tlsclient.WithProxyUrl(proxy))
	}
	c, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
	if err != nil {
		panic(err)
	}
	return &client{HttpClient: c}
}
