package rawdb

// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Has(key []byte) (bool, error)
	// Get returns the data for key, or an error. Must return common.ErrNotFound when not found.
	Get(key []byte) ([]byte, error)
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Put(key []byte, value []byte) error
}

// DatabaseDeleter wraps the Delete method of a backing data store.
type DatabaseDeleter interface {
	Delete(key []byte) error
}
