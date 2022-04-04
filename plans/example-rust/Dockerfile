FROM rust:1.57-bullseye as builder
WORKDIR /usr/src/testplan

# Cache dependencies between test runs,
# See https://blog.mgattozzi.dev/caching-rust-docker-builds/
# And https://github.com/rust-lang/cargo/issues/2644

RUN mkdir -p ./plan/src/
COPY ./plan/empty.rs ./plan/src/main.rs
COPY ./plan/Cargo.lock ./plan/
COPY ./plan/Cargo.toml ./plan/
RUN cd ./plan/ && cargo build --release

COPY . .
RUN cd ./plan/ && cargo build --release && cargo install --path .

FROM debian:bullseye-slim
COPY --from=builder /usr/local/cargo/bin/testplan /usr/local/bin/testplan
EXPOSE 6060
ENTRYPOINT ["testplan"]