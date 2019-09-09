# GO_VERSION is the golang version this image will be built against.
ARG GO_VERSION
# TESTPLAN_EXEC_PKG is the executable package of the testplan to build.
# The image will build that package only.
ARG TESTPLAN_EXEC_PKG

# Dynamically select the golang version.
# TODO: Not sure how this interplays with image caching.
FROM golang:${GO_VERSION}-buster

# Testground source code, **with a modified go.mod** pinning the upstream
# dependencies we're testing against. Until we define a plugin architecture for
# test plans, we need the full thing here, as the test plans themselves use
# packages from the testground.
ENV TEST_DIR /testground
COPY . ${TEST_DIR}/

RUN cd ${TEST_DIR} \
    && go env -w GOPROXY=direct \
    && go mod download \
	&& go build -o testplan ${TESTPLAN_EXEC_PKG}
	
FROM busybox:1.31.0-glibc
ENV SRC_DIR /testground

COPY --from=0 ${SRC_DIR}/testplan /

ENTRYPOINT [ "/testplan" ]