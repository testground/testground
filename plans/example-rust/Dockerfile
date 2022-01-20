FROM rust:1.57-bullseye as builder
WORKDIR /usr/src/testplan
COPY . .
RUN cargo install --path plan

FROM debian:bullseye-slim
COPY --from=builder /usr/local/cargo/bin/testplan /usr/local/bin/testplan
EXPOSE 6060
ENTRYPOINT [ "testplan"]