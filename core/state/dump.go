package state

import (
	"encoding/json"
	"fmt"

	"github.com/ChainAAS/gendchain/common"
	"github.com/ChainAAS/gendchain/rlp"
	"github.com/ChainAAS/gendchain/trie"
)

type DumpAccount struct {
	Balance  string            `json:"balance"`
	Nonce    uint64            `json:"nonce"`
	Root     string            `json:"root"`
	CodeHash string            `json:"codeHash"`
	Code     string            `json:"code"`
	Storage  map[string]string `json:"storage"`
}

type Dump struct {
	Root     string                 `json:"root"`
	Accounts map[string]DumpAccount `json:"accounts"`
}

func (db *StateDB) RawDump() Dump {
	dump := Dump{
		Root:     fmt.Sprintf("%x", db.trie.Hash()),
		Accounts: make(map[string]DumpAccount),
	}

	it := trie.NewIterator(db.trie.NodeIterator(nil))
	for it.Next() {
		addr := db.trie.GetKey(it.Key)
		var data Account
		if err := rlp.DecodeBytes(it.Value, &data); err != nil {
			panic(err)
		}

		obj := newObject(nil, common.BytesToAddress(addr), data)
		account := DumpAccount{
			Balance:  data.Balance.String(),
			Nonce:    data.Nonce,
			Root:     common.Bytes2Hex(data.Root[:]),
			CodeHash: common.Bytes2Hex(data.CodeHash[:]),
			Code:     common.Bytes2Hex(obj.Code(db.db)),
			Storage:  make(map[string]string),
		}
		storageIt := trie.NewIterator(obj.getTrie(db.db).NodeIterator(nil))
		for storageIt.Next() {
			account.Storage[common.Bytes2Hex(db.trie.GetKey(storageIt.Key))] = common.Bytes2Hex(storageIt.Value)
		}
		dump.Accounts[common.Bytes2Hex(addr)] = account
	}
	return dump
}

func (db *StateDB) Dump() []byte {
	json, err := json.MarshalIndent(db.RawDump(), "", "    ")
	if err != nil {
		fmt.Println("dump err", err)
	}

	return json
}
