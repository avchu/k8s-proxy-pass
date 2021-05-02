# k8s-proxy-pass

[![Go Report Card](https://goreportcard.com/badge/github.com/avchu/k8s-proxy-pass)](https://goreportcard.com/report/github.com/avchu/k8s-proxy-pass) ![build](https://github.com/avchu/k8s-proxy-pass/actions/workflows/go.yml/badge.svg)


K8S Tool to proxy-pass services from kubernetes cluster if you don't have a vpn to this cluster

# Problem

You have a multiplr `HTTP` endpoints at kubernetes cluster. To working with them as a Developer or QA you need to have a network access to them.
Let's assume that you have only kubectl access (no vpn or other things). To achieve that you need to run multile commands `kubectl port-forward`

This tool could save your time and terminals. It works like `nginx proxy-pass`.

`k8ssvcproxy` will *autodiscover all services* that have an edpoint and create a proxy tunnel to them using native `kubectl` tool

Example:
Considering that you have 3 Services in k8s cluester
- nginx-1.default.svc.cluster.local at 8080
- nginx-2.default.svc.cluster.local at 8280
- nginx-3.default.svc.cluster.local at 8080

`k8ssvcproxy` will discover all this SVC and will create proxys to them
| Source | Destination |
|---|---|
| localhost:8080 + Host: nginx-1.default.svc.cluster.local | nginx-1.default.svc.cluster.local at 8080 |
| localhost:8080 + Host: nginx-2.default.svc.cluster.local | nginx-1.default.svc.cluster.local at 8280 |
| localhost:8080 + Host: nginx-3.default.svc.cluster.local | nginx-1.default.svc.cluster.local at 8080 |

# Building
``` go build ```

# Ussage

```
./k8ssvcproxy --namespace default
```

Output

```
Connecting to default using /Users/avchu/.kube/config
Service to proxy: nginx
Forwarding nginx: 8080
Trying to register servce: nginx
Forwarding from 127.0.0.1:59761 -> 8080
Servce nginx registered on port 59761
```

Then you clould use `curl` to reach the endpoint
```
curl -v http://nginx.default.svc.cluster.local:8080
*   Trying 127.0.0.1...
* TCP_NODELAY set
* Connected to nginx.default.svc.cluster.local (127.0.0.1) port 8080 (#0)
> GET / HTTP/1.1
> Host: nginx.default.svc.cluster.local:8080
> User-Agent: curl/7.64.1
> Accept: */*
>
< HTTP/1.1 404 Not Found
< Cache-Control: no-cache, no-store, max-age=0, must-revalidate
< Content-Language: en
< Content-Type: text/html;charset=utf-8
< Date: Sat, 01 May 2021 12:39:24 GMT
< Expires: 0
< Pragma: no-cache
< Referrer-Policy: no-referrer
< Transfer-Encoding: chunked
<
* Connection #0 to host nginx.default.svc.cluster.local left intact
<!doctype html><html lang="en"><head><title>HTTP Status 404 – Not Found</title><style type="text/css">body {font-family:Tahoma,Arial,sans-serif;} h1, h2, h3, b {color:white;background-color:#525D76;} h1 {font-size:22px;} h2 {font-size:16px;} h3 {font-size:14px;} p {font-size:12px;} a {color:black;} .line {height:1px;background-color:#525D76;border:none;}</style></head><body><h1>HTTP Status 404 – Not Found</h1></body></html>* Closing connection 0
```
# Parameters
| Name| Description | Default value |
|---|---|---|
| namesapce | k8s namesapce to discover | `default` |
| listen | host to bind | `127.0.0.1`  |
| port | port to bind | 8080 |
| kubeconfig | path to kubeconfig| `~/.kube/config` |
