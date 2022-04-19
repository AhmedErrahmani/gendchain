
package state

import (
	"bytes"

	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/rlp"
	"github.com/ChainAAS/gendchain/trie"
)

// NewStateSync create a new state trie download scheduler.
func NewStateSync(root common.Hash, database trie.DatabaseReader) *trie.Sync {
	var syncer *trie.Sync
	callback := func(leaf []byte, parent common.Hash) error {
		var obj Account
		if err := rlp.Decode(bytes.NewReader(leaf), &obj); err != nil {
			return err
		}
		syncer.AddSubTrie(obj.Root, 64, parent, nil)
		syncer.AddRawEntry(obj.CodeHash, 64, parent)
		return nil
	}
	syncer = trie.NewSync(root, database, callback)
	return syncer
}
