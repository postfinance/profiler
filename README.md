[![Go Report Card](https://goreportcard.com/badge/github.com/postfinance/profiler)](https://goreportcard.com/report/github.com/postfinance/profiler)
[![GoDoc](https://godoc.org/github.com/postfinance/profiler?status.svg)](https://godoc.org/github.com/postfinance/profiler)
[![Build](https://github.com/postfinance/profiler/actions/workflows/build.yml/badge.svg)](https://github.com/postfinance/profiler/actions/workflows/build.yml)
[![Coverage](https://coveralls.io/repos/github/postfinance/profiler/badge.svg?branch=master)](https://coveralls.io/github/postfinance/profiler?branch=master)

# profiler

## Usage

Add the following line to your Go code:

```go
// create and start the profiler handler
profiler.New().Start()

// ... or with custom values
profiler.New(
    profiler.WithSignal(syscall.SIGUSR1),
    profiler.WithAddress(":8080"),
    profiler.WithTimeout(15 * time.Minute),
)
```

## Defaults

| Parameter | Default   |
|-----------|-----------|
| Signal    | *SIGUSR1* |
| Listen    | *:6666*   |
| Timeout   | *30m*     |

### Start the pprof endpoint

```shell
pkill -SIGUSR1 <your Go program>
```

> After *timeout* the endpoint will shutdown.

### Collect pprof data

```shell
go tool pprof -http $(hostname):8080 http://localhost:6666/debug/pprof/profile
```

... or ...

```shell
go tool pprof -http localhost:7007 http://localhost:8080/debug/pprof/profile
```

## Kubernetes

### Start the pprof endpoint

```shell
kubectl get pods
NAME                    READY   STATUS    RESTARTS   AGE
...

kubectl exec -ti <your pod> sh
/ # pkill -SIGUSR1 <your Go program>
/ #
```

> After *timeout* the endpoint will shutdown.

### Check log

```shell
kubectl logs <your pod> -f | grep 'start debug endpoint'
```

### Port-forward

```shell
kubectl port-forward <your pod> 8080:6666
Forwarding from 127.0.0.1:8080 -> 6666
Forwarding from [::1]:8080 -> 6666
Handling connection for 8080
```

### Collect pprof data

```shell
go tool pprof -http $(hostname):8888 http://localhost:8080/debug/pprof/profile
```

... or ...

```shell
go tool pprof -http localhost:7007 http://localhost:8080/debug/pprof/profile
```
