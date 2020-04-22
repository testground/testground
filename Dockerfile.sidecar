#:::
#::: BUILD CONTAINER
#:::

# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION=1.14

# Dynamically select the golang version.
FROM golang:${GO_VERSION}-buster

COPY /go.mod /go.mod

# Download deps.
RUN cd / && go mod download

# Now copy the rest of the source and run the build.
COPY . /
RUN cd / && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o testground

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
