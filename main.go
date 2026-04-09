package main

import (
	"flag"
	"io"

	"github.com/ramsrib/kluster-compare/cmd"

	"k8s.io/klog/v2"
)

func main() {
	// Silence client-go's klog so it doesn't leak into the TUI.
	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")
	flag.Set("stderrthreshold", "FATAL")
	flag.Parse()
	klog.SetOutput(io.Discard)

	cmd.Execute()
}
