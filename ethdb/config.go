package ethdb

// Configuration defaults.
const (
	DefaultMaxOpenSegmentCount = 10
)

type Config struct {
	// S3 archive options
	Endpoint        string `toml:",omitempty"`
	Bucket          string `toml:",omitempty"`
	AccessKeyID     string `toml:",omitempty"`
	SecretAccessKey string `toml:",omitempty"`

	// Per-table LRU cache settings.
	MaxOpenSegmentCount int `toml:",omitempty"`
}

// NewConfig returns a new instance of Config with defaults set.
func NewConfig() Config {
	return Config{
		MaxOpenSegmentCount: DefaultMaxOpenSegmentCount,
	}
}
