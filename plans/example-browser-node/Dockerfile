# Get the base image of Node version 16
FROM node:16

# Get the latest version of Playwright
FROM mcr.microsoft.com/playwright:v1.25.2-focal

RUN apt-get update && \
    apt-get -y install \
    libnss3 libatk-bridge2.0-0 libdrm-dev \
    libxkbcommon-dev libgbm-dev libasound-dev \
    libatspi2.0-0 libxshmfence-dev && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /plan

COPY ./plan/package*.json ./plan/

WORKDIR /plan
RUN npm ci

COPY ./plan/*.js /plan/
COPY ./plan/src /plan/src
COPY ./plan/server /plan/server
COPY ./plan/scripts /plan/scripts

RUN scripts/polyfill.sh
RUN npm run webpack
RUN npm cache clean --force

COPY ./plan/browser /plan/browser

ARG CHROME_DEBUG_PORT=9222
ARG HALT_BROWSER_ON_FINISH=false

EXPOSE ${CHROME_DEBUG_PORT}

ENV CHROME_DEBUG_PORT=${CHROME_DEBUG_PORT}
ENV HALT_BROWSER_ON_FINISH=${HALT_BROWSER_ON_FINISH}

ENTRYPOINT [ "npm", "run", "start:browser" ]
