package kube

import (
	"regexp"
	"strings"
)

// hexPattern matches 7+ character hex segments that look like hashes.
// Matches things like: 19e0eebb, 754f3ff2, 4ad47d30, e9ef0105
var hexPattern = regexp.MustCompile(`-[0-9a-f]{7,}`)

// canonicalName strips hex hash segments from a resource name to produce
// a stable key for fuzzy matching.
//
// Examples:
//
//	apoxy-gateway-19e0eebb-backplane -> apoxy-gateway-backplane
//	daily-proxy-754f3ff2            -> daily-proxy
//	cert-manager-4ad47d30-cainjector -> cert-manager-cainjector
//	lambda-proxy-e7c6f501           -> lambda-proxy
//	sidecar-services-f93f6a9a       -> sidecar-services
//	api-deployment                  -> api-deployment (unchanged)
func canonicalName(name string) string {
	result := hexPattern.ReplaceAllString(name, "")
	// Clean up any trailing or double dashes left behind
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.TrimRight(result, "-")
	return result
}
