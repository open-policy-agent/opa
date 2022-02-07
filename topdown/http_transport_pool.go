package topdown

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/logging"
)

type HttpTransportPool struct {
	enabled             bool
	maxIdleConns        int
	maxConnsPerHost     int
	maxIdleConnsPerHost int
	idleConnTimeout     time.Duration
	pool                sync.Map
	logger              logging.Logger
}

func NewPool() *HttpTransportPool {
	fmt.Printf("New pooll!!!!\n")
	return &HttpTransportPool{
		enabled:             true,
		maxIdleConns:        100,
		maxConnsPerHost:     100,
		maxIdleConnsPerHost: 100,
		idleConnTimeout:     90 * time.Second,
		pool:                sync.Map{},
		logger:              logging.New(),
	}
}

func (pool *HttpTransportPool) GetOrCreateTransport(tlsConfig *tls.Config, url *url.URL, parsedQuery *url.Values) *http.Transport {
	if !pool.enabled {
		return pool.createTransport(tlsConfig, url, parsedQuery)
	}

	key := connectionParamsKey(tlsConfig, url, parsedQuery)
	fmt.Printf("key %s\n", key)

	var tr *http.Transport
	cachedTr, ok := pool.pool.Load(key)
	if !ok {
		tr = pool.createTransport(tlsConfig, url, parsedQuery)
		pool.pool.Store(key, tr)
	} else {
		tr = cachedTr.(*http.Transport)
	}

	return tr
}

func (pool *HttpTransportPool) createTransport(tlsConfig *tls.Config, url *url.URL, parsedQuery *url.Values) *http.Transport {
	pool.logger.Debug("Creating a new transport for %s\n", url)

	tr := http.DefaultTransport.(*http.Transport).Clone()

	if url != nil && parsedQuery != nil {
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return http.DefaultTransport.(*http.Transport).DialContext(ctx, url.Scheme, parsedQuery.Get("socket"))
		}
	}

	tr.TLSClientConfig = tlsConfig
	tr.DisableKeepAlives = !pool.enabled
	// tr.MaxIdleConns = pool.maxIdleConns
	// tr.MaxConnsPerHost = pool.maxConnsPerHost
	// tr.MaxIdleConnsPerHost = pool.maxIdleConnsPerHost

	return tr
}

func connectionParamsKey(tlsConfig *tls.Config, url *url.URL, parsedQuery *url.Values) string {
	var keyBuilder strings.Builder

	if tlsConfig != nil {
		var tlsByteBuffer bytes.Buffer
		encoder := gob.NewEncoder(&tlsByteBuffer)
		encoder.Encode(tlsConfig)
		keyBuilder.Write(tlsByteBuffer.Bytes())

	}
	if url != nil {
		keyBuilder.WriteString(url.Scheme)
		keyBuilder.WriteString(url.Hostname())
	}
	if parsedQuery != nil {
		keyBuilder.WriteString(parsedQuery.Get("socket"))
	}
	return keyBuilder.String()
}
