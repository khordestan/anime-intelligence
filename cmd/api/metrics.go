package main

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
)

// A separate numeric loopback listener keeps metrics outside the public router.
func requestMetricsServer(address string) (*http.Server, net.Listener, *httpapi.RequestMetrics, error) {
	if address == "" {
		return nil, nil, nil, nil
	}
	addr, err := netip.ParseAddrPort(address)
	if err != nil || !addr.Addr().IsLoopback() || addr.Addr().Zone() != "" || addr.Port() == 0 {
		return nil, nil, nil, errors.New("HTTP_METRICS_ADDR must be a numeric loopback address and port")
	}

	listener, err := net.Listen("tcp", addr.String())
	if err != nil {
		return nil, nil, nil, errors.New("could not bind HTTP metrics listener")
	}
	metrics := &httpapi.RequestMetrics{}
	server := &http.Server{
		Handler: metrics.Handler(), ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second,
	}
	return server, listener, metrics, nil
}
