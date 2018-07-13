package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/SongLiangChen/grpc_pool"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func Dialfunc(addr string, opts ...grpc.DialOption) (*grpc_pool.IdleClient, error) {
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	c := NewGreeterClient(conn)
	return grpc_pool.NewIdleClient(conn, c), nil
}

func main() {
	var pool = grpc_pool.NewGRpcClientPool("127.0.0.1:8080", []grpc.DialOption{grpc.WithInsecure()}, Dialfunc, 5, time.Second*10)
	defer pool.Release()

	wg := sync.WaitGroup{}
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			if c, err := pool.Get(); err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				r, err := c.Client.(GreeterClient).SayHello(ctx, &HelloRequest{Name: "SongLiangChen"})
				if err != nil {
					pool.DelErrorClient(c)

				} else {
					fmt.Println(r.Message)
					pool.Put(c)
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()
}
