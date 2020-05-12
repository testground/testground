ARG build_image
ARG run_image

FROM ${build_image} AS builder
WORKDIR /plan
COPY . .
RUN go build -o /testplan

FROM ${run_image}
COPY --from=builder /testplan /testplan
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
