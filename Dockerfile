# ----- Frontend build ----- #
FROM node:12-slim AS godometer-frontend

ENV PARSE_TEMPLATE_VERSION=v1.0.0 \
    PARSE_TEMPLATE_HASH=8d1dc39e701b938f4874f3f8130cd3a324e7fa4697af36541918f9398dd61223

WORKDIR /src/go/src/github.com/lietu/godometer

# Set up dependencies
RUN set -eu \
 && apt-get update \
 && apt-get install -y curl coreutils git \
 && curl -L -o /src/parse-template https://github.com/Lieturd/parse-template/releases/download/${PARSE_TEMPLATE_VERSION}/parse-template-linux-amd64 \
 && echo "${PARSE_TEMPLATE_HASH}  /src/parse-template" | sha256sum -c

# Copy big dependencies
ADD frontend/package.json frontend/yarn.lock frontend/

# Prepare the frontend dependencies
RUN set -exu \
 && cd frontend \
 && yarn install

# Then do the actual build
ADD frontend frontend/
RUN cd frontend \
 && yarn run build

# ----- Server build ----- #
FROM golang:1.15.0-alpine AS godometer-server

WORKDIR /src/go/src/github.com/lietu/godometer

# Copy over everything for the server
ADD cmd cmd/
ADD server server/
ADD vendor vendor/
ADD *.go go.mod ./

# And build it
RUN set -exu \
 && cd cmd/godoserv \
 && go build godoserv.go

# ----- Runtime environment ----- #
FROM nginx:stable-alpine AS godometer-runtime

COPY --from=godometer-frontend /src/parse-template /usr/bin/parse-template
COPY --from=godometer-frontend /src/go/src/github.com/lietu/godometer/frontend/public /src/frontend/public/
COPY --from=godometer-server /src/go/src/github.com/lietu/godometer/cmd/godoserv/godoserv /src/cmd/godoserv/

WORKDIR /src/cmd/godoserv/
ENTRYPOINT ["./godoserv"]
