package kube

import "testing"

func TestCanonicalName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"apoxy-gateway-19e0eebb-backplane", "apoxy-gateway-backplane"},
		{"daily-proxy-754f3ff2", "daily-proxy"},
		{"cert-manager-4ad47d30-cainjector", "cert-manager-cainjector"},
		{"lambda-proxy-e7c6f501", "lambda-proxy"},
		{"sidecar-services-f93f6a9a", "sidecar-services"},
		{"traffic-control-plane-e9ef0105", "traffic-control-plane"},
		{"api-deployment", "api-deployment"},
		{"bullish-deployment", "bullish-deployment"},
		{"argocd-redis", "argocd-redis"},
		{"cert-manager-4be41ddd-webhook", "cert-manager-webhook"},
	}

	for _, tt := range tests {
		got := canonicalName(tt.input)
		if got != tt.want {
			t.Errorf("canonicalName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
