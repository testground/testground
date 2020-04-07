#:::
#::: BUILD CONTAINER
#:::

# Dynamically select the golang version.
FROM golang:1.14-buster

ARG GOPROXY

# Unfortunately there's no way to specify a ** glob pattern to cover all go.mods
# inside sdk.
COPY /sdk/sync/go.mod /sdk/sync/go.mod
COPY /sdk/runtime/go.mod /sdk/runtime/go.mod
COPY /go.mod /go.mod

# Download deps.
RUN cd / && \
    go env -w GOPROXY=${GOPROXY} && \
    go mod download

# Now copy the rest of the source and run the build.
COPY . /
RUN cd / && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o testground

#:::
#::: RUNTIME CONTAINER
#:::

FROM debian:buster

RUN apt update && apt install -y iptables
RUN mkdir -p /usr/local/bin
COPY --from=0 /testground /usr/local/bin/testground
ENV PATH="/usr/local/bin:${PATH}"

EXPOSE 6060

ENTRYPOINT [ "/usr/local/bin/testground" ]
