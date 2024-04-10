# Riak Node Exporter

Prometheus exporter for Riak node metrics. This is still a work in progress, but it already
provides everything you need to start exporting riak metrics to Prometheus.

## Building and running

### Build

```
make
```

### Running

```
./riak_exporter <flags>
```

### Flags

Name                                       | Description
-------------------------------------------|--------------------------------------------------------------------------------------------------
web.listen-address                         | Address to listen on for web interface and telemetry. (Default `:9203`)
riak.uri                                   | The URI which the Riak HTTP API listens on. (Default `http://localhost:8098`)
