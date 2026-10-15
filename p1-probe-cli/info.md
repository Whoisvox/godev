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

# v2
## StepA 5 pre experiments
1. exp1
*输出42，我自己改成了，现在启2个goroutine写ch，第一个1，第二个2(理论上goroutine启动顺序不定，不过我执行了10次都是输出2)，而且我猜测是go func启动慢于fmt.Println，因此先fmt.Println被阻塞，直到ch被goroutine写入，我得到顺序总是2 1，而不是1 2。*
2. exp2
程序直接报错死锁，向没写入的无缓冲频道读取，fatal error: all goroutines are asleep - deadlock!
goroutine 1 [chan send]:
main.main()
        /data/godev/p1-probe-cli/experiments/exp2.go:7 +0x28
exit status 2
root@vox-174:experiments#
3. exp3
和上面一样死锁，卡在超出容量的下一个
4. exp4
顺序逐行打印1-5，最后all done。去掉close(jobs)，则还是会打印1-5，但是不会打印all done，并且报错死锁
5. exp5
逐行输出1-9的2倍结果，无序的。

*wg.Wait()和close(results)放单独的gouroutine，如果在主goroutine，当results容量不够时，woker要继续往results插入，只能等主goroutine的消费results，而消费之前需要wg.wait()也就是所有worker都退出，主goroutine自己等自己，于是死锁。此外，我尝试将wg.wait()和close(results)移到主goroutine读取完results之后，发现所有jobs都输出了results，但是最后还是会死锁，因为主goroutine的读取results的for循环在等自己后边的close(results)*
*向已经close的channel发送会panic，panic: send on closed channel，不过有趣的是，读取会读到默认初始化值*

## step B
1. 我改造了probe代码，没有用results的channel，我直接一个[]Response收，我想着多一个results的channel还要多一次中转，多申请一点空间。

root@vox-174:p1-probe-cli# go run cmd/probe-cli/main.go run -c targets.yaml --concurrency 3

[{"url":"https://httpbin.org/headers","status_code":200,"latency":"916.830863ms"},{"url":"https://httpbin.org/user-agent","status_code":200,"latency":"918.488784ms"},{"url":"https://httpbin.org/get","status_code":200,"latency":"1.130991733s"},{"url":"https://httpbin.org/ip","status_code":200,"latency":"213.968412ms"},{"url":"https://httpbin.org/uuid","status_code":200,"latency":"655.027273ms"},{"url":"https://httpbin.org/base64/SGVsbG8gV29ybGQ=","status_code":200,"latency":"441.07029ms"},{"url":"https://httpbin.org/json","status_code":200,"latency":"439.424042ms"},{"url":"https://httpbin.org/robots.txt","status_code":200,"latency":"215.18178ms"},{"url":"https://httpbin.org/delay/1","status_code":200,"latency":"1.215803528s"},{"url":"https://httpbin.org/delay/2","status_code":200,"latency":"2.215628163s"},{"url":"https://httpbin.org/redirect/1","status_code":200,"latency":"528.275675ms"},{"url":"https://httpbin.org/delay/3","latency":"3.00120501s","err":"Get \"https://httpbin.org/delay/3\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"url":"https://httpbin.org/redirect/3","status_code":200,"latency":"860.196034ms"},{"url":"https://httpbin.org/delay/5","latency":"3.000953352s","err":"Get \"https://httpbin.org/delay/5\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"url":"https://httpbin.org/status/200","status_code":200,"latency":"259.602604ms"},{"url":"https://httpbin.org/redirect-to?url=https://example.com","status_code":200,"latency":"1.319753763s"},{"url":"https://httpbin.org/status/404","status_code":404,"latency":"653.574818ms"},{"url":"https://httpbin.org/status/500","status_code":500,"latency":"418.844532ms"},{"url":"https://httpbin.org/status/503","status_code":503,"latency":"214.530235ms"},{"url":"https://thissitedoesnotexist-12345.com","latency":"19.637712ms","err":"Get \"https://thissitedoesnotexist-12345.com\": dial tcp: lookup thissitedoesnotexist-12345.com on 127.0.0.53:53: no such host"},{"url":"https://httpbin.org/redirect/10","latency":"3.00140225s","err":"Get \"https://httpbin.org/relative-redirect/5\": context deadline exceeded (Client.Timeout exceeded whileawaiting headers)"},{"url":"https://httpbin.org/bytes/1024","status_code":200,"latency":"220.781463ms"},{"url":"https://invalid.tld.xyz","latency":"1.100200399s","err":"Get \"https://invalid.tld.xyz\": remote error: tls: unrecognized name"},{"url":"https://httpbin.org/bytes/10240","status_code":200,"latency":"668.924718ms"},{"url":"https://httpbin.org/stream/5","status_code":200,"latency":"643.209115ms"},{"url":"https://example.com","status_code":200,"latency":"215.799493ms"},{"url":"https://youcantfindme.com","latency":"3.000570293s","err":"Get \"https://youcantfindme.com\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"url":"https://example.org","status_code":200,"latency":"1.723635328s"}]
root@vox-174:p1-probe-cli# 

2. 由于多个goroutine竞争一个slice，出现了data race。改用channel。同时在probe的All函数里自定义client，每一次调用创建一个client，之后一直复用这个，直到新调All。jobs也传了target了，response带上name。并发1时，时间27.4s，并发10时，时间4.12s。
root@vox-174:p1-probe-cli# go run -race ./cmd/probe-cli/main.go run -c targets.yaml -n 10
[{"name":"httpbin-user-agent","url":"https://httpbin.org/user-agent","status_code":200,"latency":"1.031638921s"},{"name":"httpbin-ip","url":"https://httpbin.org/ip","status_code":200,"latency":"1.23889548s"},{"name":"httpbin-base64","url":"https://httpbin.org/base64/SGVsbG8gV29ybGQ=","status_code":200,"latency":"1.23988489s"},{"name":"httpbin-json","url":"https://httpbin.org/json","status_code":200,"latency":"1.240799676s"},{"name":"httpbin-get","url":"https://httpbin.org/get","status_code":200,"latency":"1.243476583s"},{"name":"httpbin-uuid","url":"https://httpbin.org/uuid","status_code":200,"latency":"1.661940679s"},{"name":"httpbin-headers","url":"https://httpbin.org/headers","status_code":200,"latency":"1.243616607s"},{"name":"httpbin-robots","url":"https://httpbin.org/robots.txt","status_code":200,"latency":"1.675893951s"},{"name":"status-200","url":"https://httpbin.org/status/200","status_code":200,"latency":"222.449872ms"},{"name":"redirect-1","url":"https://httpbin.org/redirect/1","status_code":200,"latency":"644.785804ms"},{"name":"status-404","url":"https://httpbin.org/status/404","status_code":404,"latency":"210.065621ms"},{"name":"status-500","url":"https://httpbin.org/status/500","status_code":500,"latency":"210.325157ms"},{"name":"not-exist-2","url":"https://thissitedoesnotexist-12345.com","latency":"19.089757ms","err":"Get \"https://thissitedoesnotexist-12345.com\": dial tcp: lookup thissitedoesnotexist-12345.com on 127.0.0.53:53:no such host"},{"name":"slow-1s","url":"https://httpbin.org/delay/1","status_code":200,"latency":"2.245120767s"},{"name":"not-exist-3","url":"https://invalid.tld.xyz","latency":"584.714018ms","err":"Get \"https://invalid.tld.xyz\": remote error: tls: unrecognized name"},{"name":"status-503","url":"https://httpbin.org/status/503","status_code":503,"latency":"882.782646ms"},{"name":"bytes-1kb","url":"https://httpbin.org/bytes/1024","status_code":200,"latency":"521.862505ms"},{"name":"slow-2s","url":"https://httpbin.org/delay/2","latency":"3.000883566s","err":"Get \"https://httpbin.org/delay/2\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"name":"example-com","url":"https://example.com","status_code":200,"latency":"567.813688ms"},{"name":"redirect-to-https","url":"https://httpbin.org/redirect-to?url=https://example.com","status_code":200,"latency":"1.854664222s"},{"name":"bytes-10kb","url":"https://httpbin.org/bytes/10240","status_code":200,"latency":"1.136925644s"},{"name":"stream-5","url":"https://httpbin.org/stream/5","status_code":200,"latency":"1.068097074s"},{"name":"slow-3s","url":"https://httpbin.org/delay/3","latency":"3.000429415s","err":"Get \"https://httpbin.org/delay/3\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"name":"redirect-3","url":"https://httpbin.org/redirect/3","status_code":200,"latency":"2.804687039s"},{"name":"slow-5s","url":"https://httpbin.org/delay/5","latency":"3.001012756s","err":"Get \"https://httpbin.org/delay/5\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"name":"redirect-loop","url":"https://httpbin.org/redirect/10","latency":"3.000876271s","err":"Get \"https://httpbin.org/relative-redirect/4\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"},{"name":"example-org","url":"https://example.org","status_code":200,"latency":"1.522088528s"},{"name":"not-exist-1","url":"https://youcantfindme.com","latency":"3.001551686s","err":"Get \"https://youcantfindme.com\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"}]

**respSet我也修了，声明和初始化放一起**
root@vox-174:p1-probe-cli# golangci-lint run ./...
cmd/probe-cli/main.go:36:4: S1021: should merge variable declaration with assignment on next line(gosimple)
                        var respSet []probe.Response
                        ^
root@vox-174:p1-probe-cli# 
root@vox-174:p1-probe-cli# 
root@vox-174:p1-probe-cli# golangci-lint run ./...
root@vox-174:p1-probe-cli# 