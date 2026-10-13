# v0
## first commit
1. go vet ./... 无输出
2. 5个目标都有输出
root@vox-174:p1-probe-cli# go run main.go
2026/10/12 11:09:15 https://youcantfindme.com get return failed, err Get "https://youcantfindme.com": dial tcp: lookup youcantfindme.com on 127.0.0.53:53: no such host
[{"url":"https://httpbin.org/get","status_code":200,"latency":1009547327},{"url":"https://jsonplaceholder.typicode.com/posts/1","status_code":200,"latency":1595381991},{"url":"https://youcantfindme.com","latency":15066699,"err":"Get \"https://youcantfindme.com\": dial tcp: lookup youcantfindme.com on 127.0.0.53:53: no such host"},{"url":"https://go.dev/doc/","status_code":200,"latency":610747674},{"url":"https://httpbin.org/delay/2","status_code":200,"latency":2220775281}]
3. resp.Body 我测试如果不加close，go vet正常，但是会一直阻塞。至于为什么我不知道

## 由于defer在循环内，导致main函数结束时resq.Body才关闭，高并发下影响性能，单独拆出封装成probe函数
1. go vet ./... passed
2. 5 ouputs for each url
3. While resq.Body hadn't been close, fd/connection lack, server close conn, client's socket go into CLOSE_WAIT forever, further more, if body hadn't been read, go http client didn't put current connection back to keep-alive pool, which means it imediately been Close, next reqest had to restart shake hand of tcp.

## make输出
root@vox-174:p1-probe-cli# make build && ./bin/probe-cli run -c targets.yaml
go build -trimpath -ldflags "-s -w -X main.version=1f6b05c-dirty -X main.commit=1f6b05c -X main.buildTime=2026-10-13T05:26:07Z" -o bin/probe-cli ./cmd/probe-cli
OK -> bin/probe-cli (1f6b05c-dirty 1f6b05c)
[{"url":"https://httpbin.org/get","status_code":200,"latency":"2.0698154s"},{"url":"https://youcantfindme.com","latency":"24.025331ms","err":"Get \"https://youcantfindme.com\": dial tcp: lookup youcantfindme.com on 127.0.0.53:53: no such host"},{"url":"https://httpbin.org/delay/2","status_code":200,"latency":"2.381495377s"}]