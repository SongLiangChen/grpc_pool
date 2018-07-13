package grpc_pool

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type MapPool struct {
	// pools
	pools map[string]*GRpcClientPool

	// dial function
	dialF DialFunc

	// max size of pool
	maxCount int

	// idle duration, client will be remove after idleTimeout if not be used
	idleTimeout time.Duration

	// some option
	opts []grpc.DialOption

	sync.RWMutex
}

func NewMapPool(dial DialFunc, opts []grpc.DialOption, maxCount int, idleTimeout time.Duration) *MapPool {
	return &MapPool{
		pools:       make(map[string]*GRpcClientPool),
		dialF:       dial,
		maxCount:    maxCount,
		idleTimeout: idleTimeout,
		opts:        opts,
	}
}

func (mp *MapPool) getPool(addr string) (*GRpcClientPool, error) {
	mp.RLock()
	defer mp.RUnlock()

	p, ok := mp.pools[addr]
	if !ok {
		return nil, errors.New(fmt.Sprintf("GRpcClientPool[%v] not exist", addr))
	}

	return p, nil
}

func (mp *MapPool) GetPool(addr string) *GRpcClientPool {
	p, err := mp.getPool(addr)
	if err != nil {
		p = NewGRpcClientPool(addr, mp.opts, mp.dialF, mp.maxCount, mp.idleTimeout)
		mp.Lock()
		mp.pools[addr] = p
		mp.Unlock()
	}
	return p
}

func (mp *MapPool) ReleasePool(addr string) error {
	p, err := mp.getPool(addr)
	if err != nil {
		return err
	}

	mp.Lock()
	delete(mp.pools, addr)
	mp.Unlock()

	p.Release()

	return nil
}

func (mp *MapPool) ReleaseAllPool() {
	mp.Lock()
	for _, p := range mp.pools {
		p.Release()
	}
	mp.pools = make(map[string]*GRpcClientPool)
	mp.Unlock()
}
