package ethapi

import (
	"sync"

	"github.com/ChainAAS/gendchain/common"
)

// AddrLocker stores locks per account. This is used to prevent another tx getting the
// same nonce, by holding the lock when signing a transaction.
type AddrLocker struct {
	mu    sync.RWMutex
	locks map[common.Address]*sync.Mutex
}

func newAddrLocker() *AddrLocker {
	return &AddrLocker{
		locks: make(map[common.Address]*sync.Mutex),
	}
}

// lock returns the lock of the given address.
func (l *AddrLocker) lock(address common.Address) sync.Locker {
	l.mu.RLock()
	mu, ok := l.locks[address]
	l.mu.RUnlock()
	if ok {
		return mu
	}
	l.mu.Lock()
	mu, ok = l.locks[address]
	if !ok {
		mu = new(sync.Mutex)
		l.locks[address] = mu
	}
	l.mu.Unlock()
	return mu
}
