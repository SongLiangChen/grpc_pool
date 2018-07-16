# grpc_pool
grpc_pool is a golang implementation of grpc connection pool, it create and manage connections automatic.

## proto file
We assume the proto file like below:
```
syntax = "proto3";

package main;

// The greeting service definition.
service Greeter {
  // Sends a greeting
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}

// The request message containing the user's name.
message HelloRequest {
  string name = 1;
}

// The response message containing the greetings
message HelloReply {
  string message = 1;
}
```

## Installation
Install grpc_pool with go tool:
```
    go get github.com/SongLiangChen/grpc_pool
```

## Usage
To use grpc_pool, you need import the package and design your DialFunc and create new pool instance,
The complete example is as follows:
```go
package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/SongLiangChen/grpc_pool"
)

func Dial(addr string, opts ...grpc.DialOption) (*grpc_pool.IdleClient, error) {
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	c := NewGreeterClient(conn) // Replace to your own GRpc Client Func
	return grpc_pool.NewIdleClient(conn, c), nil
}

func main() {
	pool := grpc_pool.NewGRpcClientPool("127.0.0.1:8080", []grpc.DialOption{grpc.WithInsecure()}, Dial, 5, time.Second*10)
	
	if c, err := pool.Get(); err == nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r, err := c.Client.(GreeterClient).SayHello(ctx, &HelloRequest{Name: "SongLiangChen"})
		if err != nil {
			// Invoke DelErrorClient func activity when any error happy
			pool.DelErrorClient(c)

		} else {
			fmt.Println(r.Message)
			pool.Put(c)
		}
	}
}
```
