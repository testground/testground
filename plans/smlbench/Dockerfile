# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION

# TESTPLAN_EXEC_PKG is the executable package of the testplan to build.
# The image will build that package only.
ARG TESTPLAN_EXEC_PKG

# Dynamically select the golang version.
# TODO: Not sure how this interplays with image caching.
FROM golang:${GO_VERSION}-buster

ENV TESTPLAN_EXEC_PKG ${TESTPLAN_EXEC_PKG}
COPY . /

# PLAN_DIR is the location where the plan's source code has been copied.
ENV PLAN_DIR /plan
RUN cd ${PLAN_DIR} \
    && go env -w GOPROXY=direct \
    && go mod download \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o testplan ${TESTPLAN_EXEC_PKG}

FROM busybox:1.31.0-glibc

COPY --from=0 ${PLAN_DIR}/testplan /

ADD https://dist.ipfs.io/go-ipfs/v0.4.22/go-ipfs_v${GO_IPFS_VERSION}_linux-amd64.tar.gz /
RUN tar -xjf go-ipfs_v${GO_IPFS_VERSION}_linux-amd64.tar.gz
COPY /go-ipfs/ipfs /

ENTRYPOINT [ "/testplan" ]