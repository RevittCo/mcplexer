package cache

// ServerCacheConfig holds per-downstream-server caching configuration.
type ServerCacheConfig struct {
	Enabled           bool              `json:"enabled"`
	ReadTTLSec        int               `json:"read_ttl_sec"`
	CacheablePatterns []string          `json:"cacheable_patterns"`
	MutationPatterns  []string          `json:"mutation_patterns"`
	InvalidationRules []InvalidationRule `json:"invalidation_rules,omitempty"`
	MaxEntries        int               `json:"max_entries"`
}

// InvalidationRule defines a targeted cache invalidation: when a mutation
// matching MutationPattern is called, entries matching InvalidatePattern
// for the same server+scope are evicted.
type InvalidationRule struct {
	MutationPattern   string `json:"mutation_pattern"`
	InvalidatePattern string `json:"invalidate_pattern"`
}

// DefaultCacheablePatterns are tool name prefixes that indicate read operations.
var DefaultCacheablePatterns = []string{
	"get_*", "list_*", "search_*", "read_*", "fetch_*", "query_*", "find_*",
}

// DefaultMutationPatterns are tool name prefixes that indicate write operations.
var DefaultMutationPatterns = []string{
	"create_*", "update_*", "delete_*", "send_*", "post_*",
	"put_*", "set_*", "add_*", "remove_*",
}

// DefaultServerCacheConfig returns the default caching config for a server.
func DefaultServerCacheConfig() ServerCacheConfig {
	return ServerCacheConfig{
		Enabled:           true,
		ReadTTLSec:        1800,
		CacheablePatterns: DefaultCacheablePatterns,
		MutationPatterns:  DefaultMutationPatterns,
		MaxEntries:        1000,
	}
}
