package plugins

import "time"

const (
	// DefaultTimeout is the default timeout for gRPC operations
	DefaultTimeout = 30 * time.Second

	// DefaultPluginDir is the default directory where plugins are stored
	DefaultPluginDir = "/usr/local/lib/lima/plugins"
) 