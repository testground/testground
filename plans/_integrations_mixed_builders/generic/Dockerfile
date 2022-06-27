ARG image
FROM ${image} AS builder

WORKDIR /build
ENV CGO_ENABLED 0
COPY . .

ARG plan_path
RUN cd plan/${plan_path} && go build -a -o /testplan

FROM ${image}
COPY --from=builder /testplan /testplan
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
