package main

import (
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
)

var (
	ERROR_MAX_CLIENT_COUNT = errors.New("Client count reach max count")
	ERROR_INVALID_CLIENT   = errors.New("Invalid client, maybe closed or not connected")
	ERROR_NIL_CLIENT       = errors.New("Client is nil")
)

type DialFunc func(string, ...grpc.DialOption) (*IdleClient, error)

// FOR EXAMPLE:
// func Dialfunc(addr string, opts ...grpc.DialOption) (*IdleClient, error) {
// 	conn, err := grpc.Dial(addr, opts...)
// 	if err != nil {
// 		return nil, err
// 	}

// 	c := NewExampleGRpcClient(conn)
// 	return NewIdleClient(conn, c), nil
// }

type GRpcClientPool struct {
	// pool
	pool []*IdleClient

	// dial function
	dialF DialFunc

	// max size of pool
	maxCount int
	// current count of client in pool
	count int
	// idle duration, client will be remove after idleTimeout if not be used
	idleTimeout time.Duration

	// dial addr
	addr string
	// some option
	opts []grpc.DialOption

	sync.Mutex
}

// NewGRpcClientPool create new pool
func NewGRpcClientPool(addr string, opts []grpc.DialOption, dialF DialFunc, maxCount int, idleTimeout time.Duration) *GRpcClientPool {
	return &GRpcClientPool{
		pool: make([]*IdleClient, 0),

		dialF: dialF,

		maxCount:    maxCount,
		count:       0,
		idleTimeout: idleTimeout,

		addr: addr,
		opts: append([]grpc.DialOption{}, opts...),
	}
}

type IdleClient struct {
	// last time be called
	lastCalledTime time.Time

	// true grpc client
	Client interface{}
	// socket conn
	conn *grpc.ClientConn
}

// NewIdleClient create new client
func NewIdleClient(conn *grpc.ClientConn, client interface{}) *IdleClient {
	return &IdleClient{
		Client: client,
		conn:   conn,
	}
}

// idleTimeout return true if timeout
func (c *IdleClient) idleTimeout(idle time.Duration) bool {
	if c.lastCalledTime.Add(idle).After(time.Now()) {
		return false
	}

	return true
}

func (c *IdleClient) updateLastCalledTime() {
	c.lastCalledTime = time.Now()
}

func (c *IdleClient) checkValid() error {
	state := c.conn.GetState()
	if int(state) != 2 {
		return ERROR_INVALID_CLIENT
	}

	return nil
}

func (c *IdleClient) close() {
	c.conn.Close()
}

// Get get a client from pool, if pool size if zero, then create a new conn for client
func (p *GRpcClientPool) Get() (c *IdleClient, err error) {
	p.Lock()
	defer p.Unlock()

	// del the idle timeout conn
	index := 0
	for _, c := range p.pool {
		if !c.idleTimeout(p.idleTimeout) {
			break
		} else {
			c.close()
			if p.count > 0 {
				p.count--
			}
		}
		index++
	}
	p.pool = p.pool[index:]

	if len(p.pool) == 0 { // create new conn
		if p.count >= p.maxCount {
			return nil, ERROR_MAX_CLIENT_COUNT
		}

		c, err = p.dialF(p.addr, p.opts...)
		if err != nil {
			return nil, err
		}
		c.updateLastCalledTime()

		p.count++

	} else { // get a conn from pool
		c = p.pool[0]
		p.pool = p.pool[1:]
	}

	return
}

// Put put a client to pool
func (p *GRpcClientPool) Put(c *IdleClient) error {
	if c == nil {
		return ERROR_NIL_CLIENT
	}

	p.Lock()
	defer p.Unlock()

	if err := c.checkValid(); err != nil {
		c.close()
		if p.count > 0 {
			p.count--
		}
		return ERROR_INVALID_CLIENT
	}

	c.updateLastCalledTime()
	p.pool = append(p.pool, c)

	return nil
}

// DelErrorClient if rpc request get error, you SHOULD call this func manual
func (p *GRpcClientPool) DelErrorClient(c *IdleClient) {
	if c == nil {
		return
	}

	c.close()
	p.Lock()
	if p.count > 0 {
		p.count--
	}
	p.Unlock()
}

// Release release all conn
func (p *GRpcClientPool) Release() {
	p.Lock()
	defer p.Unlock()

	for _, c := range p.pool {
		if c != nil {
			c.close()
		}
	}
	p.count = 0
	p.pool = make([]*IdleClient, 0)
}
