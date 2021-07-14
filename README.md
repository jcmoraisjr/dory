# dory

A HTTP server that responds with the hostname where it is running on. It's usually the pod name if deployed on Kubernetes and using the pod network. `dory` also accepts the time that should be wait to respond, and how much that time should be randomly changed. `dory` has metrics, so she will remember how frequently you asked.

## use cases

`dory` can be used to:

* Test blue/green load balance, comparing how many times each host appeared in the response
* Test concurrent sessions and ephemeral port exhaustion, asking `dory` to wait a few (or lots of) seconds before send the response and finish the session
* Play with Prometheus metrics

## how to use

`dory` accepts the following command-line options:

* `-listen`: changes the listening IP and port, defaults to `:8000` (port 8000 on all IP addresses) if not declared
* `-buckets`: changes the list of the response time counter buckets, default value is `0.8,1,1.2`

The following HTTP headers are accepted:

* `x-wait`: number of milliseconds to wait before send the response, defaults to respond imediately
* `x-pct`: maximum percentage to randomly increase or decrease the configured wait time, e.g. `20` means that a wait time of `1000` will in fact wait between `800ms` and `1.2s`, dafaults to zero which means to wait exactly the configured amount of time

## example

The following steps configure an environment to test the distribution of response times and concurrent sessions during a load test. The only required dependencies are `docker` and `vegeta` [(download)](https://github.com/tsenart/vegeta/releases).

Run two `dory` instances with default parameters:

```
docker run -d --name=dory1 -p 8001:8000 jcmoraisjr/dory
docker run -d --name=dory2 -p 8002:8000 jcmoraisjr/dory
```

Problem with Docker Hub?

```
docker run -d --name=dory1 -p 8001:8000 quay.io/jcmoraisjr/dory
docker run -d --name=dory2 -p 8002:8000 quay.io/jcmoraisjr/dory
```

Configure a HAProxy to load balance between these two instances:

`h.cfg`:

```
defaults
  timeout client 1m
  timeout server 1m
  timeout connect 5s
listen l
  bind :8000
  server dory1 127.0.0.1:8001
  server dory2 127.0.0.1:8002
```

```
docker run -d --name=haproxy --net=host -v $PWD:/tmp:ro haproxy:alpine -f /tmp/h.cfg
```

Configure a Prometheus server to scrape metrics from these two instances:

`prometheus.yml`:

```
global:
  scrape_interval: 10s
  scrape_timeout: 5s
scrape_configs:
- job_name: prometheus
  static_configs:
  - targets:
    - 127.0.0.1:9090
- job_name: dory
  static_configs:
  - targets:
    - 127.0.0.1:8001
    - 127.0.0.1:8002
```

```
docker run -d --name=prom --net=host -v $PWD:/etc/prometheus:ro prom/prometheus
```

Run the load test, the command below will run 100 requests per second during 5 minutes:

```
echo "GET http://127.0.0.1:8000" |\
  vegeta attack -duration=5m -rate=100 -header "x-wait: 1000" -header "x-pct: 25" |\
  vegeta report
```

The headers ask `dory` to wait between `750ms` and `1.25s` to send the response, this should fill all the buckets of our histogram, which defaults to `0.8s`, `1s` and `1.2s`.

After the load test, of even while it's still running, send the following queries to Prometheus - it should be running on `http://127.0.0.1:9090`, or maybe you need to change to the public IP of your VM.

Query the concurrent sessions - per instance:

```
dory_sessions
```

Query the percentage of response times below `0.8s` - per instance:

```
100 * sum(rate(dory_response_time_seconds_bucket{le="0.8"}[1m])) by (instance) / sum(rate(dory_response_time_seconds_count[1m])) by (instance)
```

Note that the graph won't be drawn in the example above during request rate equals to zero, this happens due to a division by zero error.

## cleanup

Need to remove all the containers and start again?

```
docker stop dory1 dory2 haproxy prom
docker rm dory1 dory2 haproxy prom
```
