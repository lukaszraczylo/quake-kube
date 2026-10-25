FROM node:22-alpine AS web

WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN mkdir -p /public && pnpm exec vite build --outDir /public --emptyOutDir

FROM golang:1 AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
ARG GOPROXY
ARG GOSUMDB
RUN go mod download

ADD cmd cmd/
ADD internal internal/
ADD public public/
COPY --from=web /public/app.js public/app.js
COPY tools tools/
RUN /usr/local/go/bin/go run ./tools/genstatic.go public public

RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on taskset -c 1 /usr/local/go/bin/go build -a -o q3 ./cmd/q3

FROM alpine:3 AS quake-n-bake

RUN apk add --no-cache git gcc g++ make cmake musl-dev linux-headers
RUN git clone --depth 1 https://github.com/ioquake/ioq3 && \
    cd /ioq3 && \
    cmake -S . -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_CLIENT=OFF \
        -DBUILD_SERVER=ON \
        -DBUILD_GAME=OFF \
        -DBUILD_MISSIONPACK=OFF \
        -DBUILD_BASEGAME=OFF \
        -DBUILD_RENDERER_OPENGL2=OFF \
        -DBUILD_STANDALONE=ON && \
    cmake --build build --parallel
# Find and copy the dedicated server binary
RUN find /ioq3/build/ -type f -name "ioq3ded*" -exec cp {} /usr/local/bin/ioq3ded \;

FROM alpine:3

COPY --from=builder /workspace/q3 /usr/local/bin
COPY --from=quake-n-bake /usr/local/bin/ioq3ded /usr/local/bin
COPY --from=quake-n-bake /lib/ld-musl-*.so.1 /lib

ENTRYPOINT ["/usr/local/bin/q3"]
