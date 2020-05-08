FROM golang:alpine AS builder
WORKDIR /plan
COPY . .
RUN go build -o /testplan

FROM alpine
COPY --from=builder /testplan /testplan
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
