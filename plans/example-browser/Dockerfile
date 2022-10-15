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
RUN npm ci && npm cache clean --force

COPY ./plan/src /plan/src

ENTRYPOINT [ "npm", "start" ]
