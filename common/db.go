package common

import (
	"errors"
	"io"
)

var ErrNotFound = errors.New("not found")

// Database wraps all database operations. All methods are safe for concurrent use.
type Database interface {
	io.Closer
	GlobalTable() Table
	BodyTable() Table
	HeaderTable() Table
	ReceiptTable() Table
}

// Putter wraps the write operation supported by both batches and regular tables.
type Putter interface {
	Put(key []byte, value []byte) error
}

// Deleter wraps the database delete operation supported by both batches and regular databases.
type Deleter interface {
	Delete(key []byte) error
}

// Table wraps all mutation & accessor operations. All methods are safe for concurrent use.
type Table interface {
	Putter
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Delete(key []byte) error
	NewBatch() Batch
}

// Batch is a write-only database that commits changes to its host database
// when Write is called. Batch cannot be used concurrently.
type Batch interface {
	Putter
	Deleter
	ValueSize() int // amount of data in the batch
	Write() error
	// Reset resets the batch for reuse
	Reset()
}

// TablePrefixer represents an wrapper for Database that prefixes all operations with a key prefix.
type TablePrefixer struct {
	table  Table
	prefix string
}

// NewTablePrefixer returns a new instance of TablePrefixer.
func NewTablePrefixer(t Table, prefix string) *TablePrefixer {
	return &TablePrefixer{table: t, prefix: prefix}
}

func (p *TablePrefixer) Put(key []byte, value []byte) error {
	return p.table.Put(append([]byte(p.prefix), key...), value)
}

func (p *TablePrefixer) Has(key []byte) (bool, error) {
	return p.table.Has(append([]byte(p.prefix), key...))
}

func (p *TablePrefixer) Get(key []byte) ([]byte, error) {
	return p.table.Get(append([]byte(p.prefix), key...))
}

func (p *TablePrefixer) Delete(key []byte) error {
	return p.table.Delete(append([]byte(p.prefix), key...))
}

func (p *TablePrefixer) Close() error { return nil }

func (p *TablePrefixer) NewBatch() Batch {
	return &TablePrefixerBatch{p.table.NewBatch(), p.prefix}
}

type TablePrefixerBatch struct {
	batch  Batch
	prefix string
}

func (b *TablePrefixerBatch) Put(key, value []byte) error {
	return b.batch.Put(append([]byte(b.prefix), key...), value)
}

func (b *TablePrefixerBatch) Delete(key []byte) error {
	return b.batch.Delete(append([]byte(b.prefix), key...))
}

func (b *TablePrefixerBatch) Write() error {
	return b.batch.Write()
}

func (b *TablePrefixerBatch) ValueSize() int {
	return b.batch.ValueSize()
}

func (b *TablePrefixerBatch) Reset() {
	b.batch.Reset()
}
