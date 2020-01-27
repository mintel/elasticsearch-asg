package throttler

// ClusterState represents the state of an Elasticsearch
// cluster in the context of the throttler app.
type ClusterState struct {
	// One of: "red", "yellow", "green".
	Status string

	// True if shards are being moved from one node to another.
	RelocatingShards bool

	// True if indices are recovering from data
	// stored on disk, such as during a node reboot.
	RecoveringFromStore bool
}
