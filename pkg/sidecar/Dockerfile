#:::
#::: BUILD CONTAINER
#:::

# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION

# Dynamically select the golang version.
# TODO: Not sure how this interplays with image caching.
FROM golang:${GO_VERSION}-buster

# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION

# Unfortunately there's no way to specify a ** glob pattern to cover all go.mods
# inside sdk.
COPY /sdk/sync/go.mod /sdk/sync/go.mod
COPY /sdk/runtime/go.mod /sdk/runtime/go.mod
COPY /go.mod /go.mod

# Download deps.
RUN cd ${PLAN_DIR} \
    && go env -w GOPROXY=direct \
    && go mod download

# Now copy the rest of the source and run the build.
COPY . /
RUN cd / \
    && go env -w GOPROXY=direct \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o testground

#:::
#::: RUNTIME CONTAINER
#:::

FROM busybox:1.31.0-glibc

RUN mkdir -p /usr/local/bin
COPY --from=0 /testground /usr/local/bin/testground
ENV PATH="/usr/local/bin:${PATH}"

ENTRYPOINT [ "/usr/local/bin/testground" ]
CMD ["sidecar"]
