package main

// grpc 连接池

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

// FOR EXAMPLE:
// func Dialfunc(addr string, opts ...grpc.DialOption) (*IdleClient, error) {
// 	conn, err := grpc.Dial(addr, opts...)
// 	if err != nil {
// 		return nil, err
// 	}

// 	c := NewExampleGRpcClient(conn)
// 	return NewIdleClient(conn, c), nil
// }
type DialFunc func(string, ...grpc.DialOption) (*IdleClient, error)

// GRpcClientPool is a pool that manage connections to rpc server.
// cache and remove idle timeout connection, and keep the conn num
// not over maxCount.
type GRpcClientPool struct {
	// Connections to rpc server
	pool []*IdleClient

	// Dial function, use to create new conn
	dialF DialFunc

	// Max size of pool
	maxCount int
	// Valid conn num in pool for now
	count int
	// Idle duration, client will be remove after idleTimeout from last used time
	idleTimeout time.Duration

	// Rpc server address
	addr string
	// Some option, see "google.golang.org/grpc.DialOption"
	opts []grpc.DialOption

	sync.Mutex
}

func NewGRpcClientPool(addr string, opts []grpc.DialOption, dialF DialFunc, maxCount int, idleTimeout time.Duration) *GRpcClientPool {
	if opts == nil || len(opts) == 0 {
		opts = []grpc.DialOption{grpc.WithInsecure()}
	}

	return &GRpcClientPool{
		pool: make([]*IdleClient, 0),

		dialF: dialF,

		maxCount:    maxCount,
		count:       0,
		idleTimeout: idleTimeout,

		addr: addr,
		opts: opts,
	}
}

// IdleClient is the implement of connection of rpc server
type IdleClient struct {
	// Last time be called
	lastCalledTime time.Time

	// True grpc client
	Client interface{}
	// Socket conn
	conn *grpc.ClientConn
}

func NewIdleClient(conn *grpc.ClientConn, client interface{}) *IdleClient {
	return &IdleClient{
		Client: client,
		conn:   conn,
	}
}

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

// Get return a valid connection of rpc server, or an error
func (p *GRpcClientPool) Get() (c *IdleClient, err error) {
	p.Lock()
	defer p.Unlock()

	// del stale conns
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

// Put give back connection to pool
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

// DelErrorClient handle an invalid connection, you SHOULD call this func manual
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
