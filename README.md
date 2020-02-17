[![Go Report Card](https://goreportcard.com/badge/github.com/postfinance/profiler)](https://goreportcard.com/report/github.com/postfinance/profiler)
[![GoDoc](https://godoc.org/github.com/postfinance/profiler?status.svg)](https://godoc.org/github.com/postfinance/profiler)
[![Build Status](https://travis-ci.org/postfinance/profiler.svg?branch=master)](https://travis-ci.org/postfinance/profiler)
[![Coverage Status](https://coveralls.io/repos/github/postfinance/profiler/badge.svg?branch=master)](https://coveralls.io/github/postfinance/profiler?branch=master)


<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [profiler](#profiler)
    - [Usage](#usage)
        - [Start the pprof endpoint](#start-the-pprof-endpoint)
        - [Collect pprof data](#collect-pprof-data)
    - [Usage with kubernetes services](#usage-with-kubernetes-services)
        - [Start the pprof endpoint](#start-the-pprof-endpoint-1)
        - [Check log](#check-log)
        - [Port-forward](#port-forward)
        - [Collect pprof data](#collect-pprof-data-1)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# profiler

## Usage

Add the following line to your Go code:
```go
// create and start the profiler handler
profiler.New().Start()

// ... or with custom values
profiler.New(
    profiler.WithSignal(syscall.SIGUSR2),
    profiler.WithAddress(":8080"),
    profiler.WithTimeout(15 * time.Minute),
)
```

Defaults:
- Signal *USR1*
- Listen *:6666*
- Timeout *10m*

### Start the pprof endpoint
```bash
pkill -USR1 <your Go program>
```
After *timeout* the endpoint will shutdown.

### Collect pprof data
```bash
go tool pprof -http $(hostname):8888 http://localhost:6666/debug/pprof/profile
```

## Usage with kubernetes services

### Start the pprof endpoint
```bash
$ k get pods
NAME                    READY   STATUS    RESTARTS   AGE
mule-8584d5dcd6-6kllk   1/1     Running   0          17m
mule-8584d5dcd6-8m89n   1/1     Running   0          17m
mule-8584d5dcd6-cvntt   1/1     Running   0          17m
$ k exec -ti mule-8584d5dcd6-6kllk sh
/ # pkill -SIGUSR1 mule-server
/ #
```
After *timeout* the endpoint will shutdown.


### Check log
```bash
$ k logs mule-8584d5dcd6-6kllk -f
...
2020/02/10 16:37:09 start pprof endpoint on ":6666"
...
```

### Port-forward
```bash
$  k port-forward mule-8584d5dcd6-6kllk 8080:6666
Forwarding from 127.0.0.1:8080 -> 6666
Forwarding from [::1]:8080 -> 6666
Handling connection for 8080
```

### Collect pprof data
```bash
$ go tool pprof -http $(hostname):8888 http://localhost:8080/debug/pprof/profile
```


