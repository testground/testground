# This is only here to test the example-plan. When the package is oficially
# released in the npm store, we can remove this and rearrange the example to
# the Testground repository

FROM node:14.12.0-buster AS builder
WORKDIR /build
ENV CGO_ENABLED 0
ENV LOG_LEVEL debug
COPY . .
RUN cd plan && npm ci
EXPOSE 6060
ENTRYPOINT [ "node", "plan/index.js"]
