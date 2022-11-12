# Get the base image of Node version 16
FROM node:16

# Get the latest version of Playwright
FROM mcr.microsoft.com/playwright:v1.27.1-focal

RUN apt-get update && \
    apt-get -y install \
    libnss3 libatk-bridge2.0-0 libdrm-dev \
    libxkbcommon-dev libgbm-dev libasound-dev \
    libatspi2.0-0 libxshmfence-dev \
    libevent-2.1-7 libenchant-2-2 \
    socat && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /plan

COPY ./plan/package*.json ./plan/

WORKDIR /plan
RUN npm ci

COPY ./plan/*.js /plan/
COPY ./plan/src /plan/src
COPY ./plan/browser /plan/browser

RUN npm run webpack
RUN npm cache clean --force

ARG TEST_BROWSER_DEBUG_PORT=9222

# TODO: delete these args once we have docker:launch runner args in master
ARG TEST_BROWSER_KIND="chromium"
ARG TEST_KEEP_OPENED_BROWSERS=false

EXPOSE ${TEST_BROWSER_DEBUG_PORT}

ENV TEST_BROWSER_KIND=${TEST_BROWSER_KIND}
ENV TEST_BROWSER_DEBUG_PORT=${TEST_BROWSER_DEBUG_PORT}
ENV TEST_KEEP_OPENED_BROWSERS=false

ENTRYPOINT [ "npm", "run", "start:browser" ]
