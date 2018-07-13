# grpc_pool
a pool of grpc client

## usage
### 创建连接池

假设pb文件如下：
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

1. 首先构建一个连接函数，当需要创建新的连接时，连接池将使用该函数
func Dial(addr string, opts ...grpc.DialOption) (*grpc_pool.IdleClient, error) {
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}

	c := NewGreeterClient(conn) // 对应你自己的rpc client
	return grpc_pool.NewIdleClient(conn, c), nil
}

2. 创建连接池
pool := grpc_pool.NewGRpcClientPool("127.0.0.1:8080", []grpc.DialOption{grpc.WithInsecure()}, Dial, 5, time.Second*10)

### 使用连接池
if c, err := pool.Get(); err == nil {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		r, err := c.Client.(GreeterClient).SayHello(ctx, &HelloRequest{Name: "SongLiangChen"})
		if err != nil {
      // 发生错误，主动调用DelErrorClient
			pool.DelErrorClient(c)

		} else {
			fmt.Println(r.Message)
			pool.Put(c)
		}
}
