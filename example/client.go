package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/SongLiangChen/grpc_pool"
	"golang.org/x/net/context"
)

func main() {
	var pool = grpc_pool.NewGRpcClientPool("127.0.0.1:8080", nil, 5, time.Second*10)
	defer pool.Release()

	wg := sync.WaitGroup{}
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			if c, err := pool.Get(); err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				r, err := NewGreeterClient(c.GetConn()).SayHello(ctx, &HelloRequest{Name: "SongLiangChen"})
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
