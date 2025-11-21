package events

// Process lifecycle events (published to /process stream)

// EventVarnishadmListening is published when the varnishadm server starts listening
type EventVarnishadmListening struct {
	Port uint16
}

// EventVarnishdStarted is published when varnishd process starts
type EventVarnishdStarted struct {
	Cmd  string
	Args []string
}

// EventVarnishdConnected is published when varnishd successfully connects to varnishadm
type EventVarnishdConnected struct {
	RemoteAddr string
}

// EventVCLLoaded is published when VCL is successfully loaded
type EventVCLLoaded struct {
	Name    string
	Path    string
	Mapping any // VCL config ID to filename mapping (varnishadm.VCLShowResult)
}

// EventCacheStarted is published when the cache process is started
type EventCacheStarted struct{}

// EventReady is published when the system is fully initialized and ready
type EventReady struct{}

// EventProcessError is published when a component encounters an error
type EventProcessError struct {
	Component string
	Error     error
}
