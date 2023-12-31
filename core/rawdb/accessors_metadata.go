package rawdb

import (
	"encoding/json"
	"fmt"

	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/log"
	"github.com/ChainAAS/gendchain/metrics"
	"github.com/ChainAAS/gendchain/params"
	"github.com/ChainAAS/gendchain/rlp"
)

var (
	preimageCounter    = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)
)

// ReadDatabaseVersion retrieves the version number of the database.
func ReadDatabaseVersion(db DatabaseReader) *uint64 {
	var version uint64

	var enc []byte
	Must("get", func() (err error) {
		enc, err = db.Get(databaseVersionKey)
		if err == common.ErrNotFound {
			err = nil
		}
		return
	})
	if err := rlp.DecodeBytes(enc, &version); err != nil {
		log.Error("Failed to decode database version", "encoded", enc)
		return nil
	}
	return &version
}

// WriteDatabaseVersion stores the version number of the database
func WriteDatabaseVersion(db DatabaseWriter, version uint64) {
	enc, err := rlp.EncodeToBytes(version)
	if err != nil {
		log.Error("Failed to encode database version", "version", version)
		return
	}
	Must("put database version", func() error {
		return db.Put(databaseVersionKey, enc)
	})
}

// ReadChainConfig retrieves the consensus settings based on the given genesis hash.
func ReadChainConfig(db DatabaseReader, hash common.Hash) *params.ChainConfig {
	var data []byte
	Must("get chain config", func() (err error) {
		data, err = db.Get(configKey(hash))
		if err == common.ErrNotFound {
			err = nil
		}
		return
	})
	if len(data) == 0 {
		return nil
	}
	var config params.ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", "hash", hash, "err", err)
		return nil
	}
	return &config
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db DatabaseWriter, hash common.Hash, cfg *params.ChainConfig) {
	if cfg == nil {
		return
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		log.Crit("Failed to JSON encode chain config", "err", err)
	}
	Must("put chain config", func() error {
		return db.Put(configKey(hash), data)
	})
}

// ReadPreimage retrieves a single preimage of the provided hash.
func ReadPreimage(db DatabaseReader, hash common.Hash) []byte {
	var data []byte
	Must("get preimage", func() (err error) {
		data, err = db.Get(preimageKey(hash))
		if err == common.ErrNotFound {
			err = nil
		}
		return
	})
	return data
}

// PreimageTablePrefixer returns a Table instance with the key prefix for preimage entries.
func PreimageTablePrefixer(tbl common.Table) common.Table {
	return common.NewTablePrefixer(tbl, preimagePrefix)
}

// WritePreimages writes the provided set of preimages to the database. `number` is the
// current block number, and is used for debug messages only.
func WritePreimages(tbl common.Table, number uint64, preimages map[common.Hash][]byte) {
	p := PreimageTablePrefixer(tbl)
	batch := tbl.NewBatch()
	hitCount := 0
	op := fmt.Sprintf("add preimage %d to batch", number)
	for hash, preimage := range preimages {
		if _, err := p.Get(hash.Bytes()); err != nil {
			Must(op, func() error {
				return batch.Put(hash.Bytes(), preimage)
			})
			hitCount++
		}
	}
	preimageCounter.Inc(int64(len(preimages)))
	preimageHitCounter.Inc(int64(hitCount))
	if hitCount > 0 {
		Must(fmt.Sprintf("write preimage %d batch", number), batch.Write)
	}
}
