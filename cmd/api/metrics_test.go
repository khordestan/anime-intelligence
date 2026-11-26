package main

import (
	"testing"
)

func TestRequestMetricsConfigurationRequiresNumericLoopback(t *testing.T) {
	if server, listener, metrics, err := requestMetricsServer(""); server != nil || listener != nil || metrics != nil || err != nil {
		t.Fatal("unset metrics configuration created a listener")
	}
	for _, address := range []string{":9091", "0.0.0.0:9091", "[::]:9091", "localhost:9091", "127.0.0.1:0", "127.0.0.1", "[::1%lo]:9091"} {
		if _, listener, _, err := requestMetricsServer(address); err == nil {
			if listener != nil {
				_ = listener.Close()
			}
			t.Errorf("accepted address %q", address)
		}
	}
}
