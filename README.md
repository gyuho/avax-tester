
**NOTE: TO BE MIGRATED TO https://github.com/ava-labs/avalanche-network-runner.**

# `network-runner` RPC server

**What does `network-runner` do?** The primary focus of [`network-runner`](https://github.com/ava-labs/avalanche-network-runner) is to create a local network, as a test framework for local development.

**Why `network-runner` as a binary?** Previously, each team was still required to write a substantial amount of Go code to integrate with `network-runner`. And the circular dependency on `avalanchego` made it unusable within `avalanchego` itself. Using `network-runner` as a binary (rather than Go package) eliminates the complexity of such dependency management.

**Why `network-runner` needs RPC server?** `network-runner` needs to provide more complex workflow such as replace, restart, inject fail points, etc.. The RPC server will expose basic node operations to enable a separation of concerns such that one team develops test framework, and the other writes test cases and its controlling logic.

**Why gRPC?** The RPC server leads to more modular test components, and gRPC enables greater flexibility. The protocol buffer increases flexibility as we develop more complicated test cases. And gRPC opens up a variety of different approaches for how to write test controller (e.g., Rust). See [`rpcpb/rpc.proto`](./rpcpb/rpc.proto) for service definition.

**Why gRPC gateway?** [gRPC gateway](https://grpc-ecosystem.github.io/grpc-gateway/) exposes gRPC API via HTTP, without us writing any code. Which can be useful if a test controller writer does not want to deal with gRPC.

## Examples

```bash
# to install
cd ${HOME}/go/src/github.com/gyuho/avax-tester
go install -v ./cmd/avalanche-network-runner
```

To start the server:

```bash
avalanche-network-runner server \
--log-level debug \
--port=":8080" \
--grpc-gateway-port=":8081"
```

To ping the server:

```bash
curl -X POST -k http://localhost:8081/v1/ping -d ''

# or
avalanche-network-runner client ping \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

To start the server:

```bash
# replace with your local path
curl -X POST -k http://localhost:8081/v1/control/start -d '{"execPath":"/Users/gyuho.lee/go/src/github.com/ava-labs/avalanchego/build/avalanchego",whitelistedSubnets:""}'

# or
avalanche-network-runner control start \
--log-level debug \
--endpoint="0.0.0.0:8080" \
--avalanchego-path ${HOME}/go/src/github.com/ava-labs/avalanchego/build/avalanchego \
--whitelisted-subnets=""
```

To wait for the cluster health:

```bash
curl -X POST -k http://localhost:8081/v1/control/health -d ''

# or
avalanche-network-runner control health \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

To get the cluster endpoints:

```bash
curl -X POST -k http://localhost:8081/v1/control/uris -d ''

# or
avalanche-network-runner control uris \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

To query the cluster status from the server:

```bash
curl -X POST -k http://localhost:8081/v1/control/status -d ''

# or
avalanche-network-runner control status \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

To stream cluster status:

```bash
avalanche-network-runner control \
--request-timeout=3m \
stream-status \
--push-interval=5s \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

To remove (stop) a node:

```bash
curl -X POST -k http://localhost:8081/v1/control/removenode -d '{"name":"node5"}'

# or
avalanche-network-runner control remove-node \
--request-timeout=3m \
--log-level debug \
--endpoint="0.0.0.0:8080" \
--node-name node5
```

To restart a node, download the test binary:

```bash
# [optional] download a binary to update
# https://github.com/ava-labs/avalanchego/releases
VERSION=1.7.3
GOARCH=$(go env GOARCH)
GOOS=$(go env GOOS)
DOWNLOAD_URL=https://github.com/ava-labs/avalanchego/releases/download/v${VERSION}/avalanchego-linux-${GOARCH}-v${VERSION}.tar.gz
DOWNLOAD_PATH=/tmp/avalanchego.tar.gz
if [[ ${GOOS} == "darwin" ]]; then
  DOWNLOAD_URL=https://github.com/ava-labs/avalanchego/releases/download/v${VERSION}/avalanchego-macos-v${VERSION}.zip
  DOWNLOAD_PATH=/tmp/avalanchego.zip
fi

rm -rf /tmp/avalanchego-v${VERSION}
rm -rf /tmp/avalanchego-build
rm -f ${DOWNLOAD_PATH}
echo "downloading avalanchego ${VERSION} at ${DOWNLOAD_URL}"
curl -L ${DOWNLOAD_URL} -o ${DOWNLOAD_PATH}

echo "extracting downloaded avalanchego"
if [[ ${GOOS} == "linux" ]]; then
  tar xzvf ${DOWNLOAD_PATH} -C /tmp
elif [[ ${GOOS} == "darwin" ]]; then
  unzip ${DOWNLOAD_PATH} -d /tmp/avalanchego-build
  mv /tmp/avalanchego-build/build /tmp/avalanchego-v${VERSION}
fi
find /tmp/avalanchego-v${VERSION}
```

To restart a node:

```bash
curl -X POST -k http://localhost:8081/v1/control/restartnode -d '{"name":"node1","startRequest":{"execPath":"/tmp/avalanchego-v1.7.3/build/avalanchego",whitelistedSubnets:""}}'

# or
avalanche-network-runner control restart-node \
--request-timeout=3m \
--log-level debug \
--endpoint="0.0.0.0:8080" \
--node-name node1 \
--avalanchego-path /tmp/avalanchego-v1.7.3/build/avalanchego \
--whitelisted-subnets=""
```

To terminate the cluster:

```bash
curl -X POST -k http://localhost:8081/v1/control/stop -d ''

# or
avalanche-network-runner control stop \
--log-level debug \
--endpoint="0.0.0.0:8080"
```

