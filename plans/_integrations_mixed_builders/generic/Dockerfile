ARG image
FROM ${image} AS builder

WORKDIR /build
ENV CGO_ENABLED 0
COPY . .

ARG PLAN_PATH
RUN cd plan/${PLAN_PATH} && go build -a -o /testplan

FROM ${image}
COPY --from=builder /testplan /testplan
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
