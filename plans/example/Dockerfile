ARG build_image
ARG run_image

FROM ${build_image} AS builder
WORKDIR /build
COPY . .
RUN cd plan && go build -o /testplan 

FROM ${run_image}
COPY --from=builder /testplan /testplan
COPY plan/artifact.txt /
EXPOSE 6060
ENTRYPOINT [ "/testplan"]
