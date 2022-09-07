FROM golang:1.16-buster AS builder
WORKDIR /build
ENV CGO_ENABLED 0

COPY . .

# issue 1170: testground ignore should skip these files
RUN if [ -f "./plan/ignore.md" ]; then echo "./plan/ignore.md should not exist"; exit 1; fi
RUN if [ ! -f "./plan/keep.md" ]; then echo "./plan/keep.md should exist"; exit 1; fi

RUN cd plan && go build -a -o /testplan

FROM busybox:1.31.1-glibc
COPY --from=builder /testplan /testplan
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
