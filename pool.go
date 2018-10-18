package grpc_pool

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
// func Dialfunc(addr string) (*grpc.ClientConn, error) {
//	return grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout())
// }
type DialFunc func(string) (*grpc.ClientConn, error)

func DefaultDialFunc(addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr, grpc.WithInsecure())
}

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

	sync.Mutex
}

func NewGRpcClientPool(addr string, dialF DialFunc, maxCount int, idleTimeout time.Duration) *GRpcClientPool {
	if dialF == nil {
		dialF = DefaultDialFunc
	}

	return &GRpcClientPool{
		pool: make([]*IdleClient, 0),

		dialF: dialF,

		maxCount:    maxCount,
		count:       0,
		idleTimeout: idleTimeout,

		addr: addr,
	}
}

// IdleClient is the implement of connection of rpc server
type IdleClient struct {
	// Last time be called
	lastCalledTime time.Time

	// Socket conn
	conn *grpc.ClientConn
}

func (c *IdleClient) GetConn() *grpc.ClientConn {
	return c.conn
}

func newIdleClient(conn *grpc.ClientConn) *IdleClient {
	return &IdleClient{
		conn: conn,
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
		if p.count >= p.maxCount && p.maxCount > 0 {
			return nil, ERROR_MAX_CLIENT_COUNT
		}

		cc, err := p.dialF(p.addr)
		if err != nil {
			return nil, err
		}
		c = newIdleClient(cc)
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
