FROM golang:1 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
ARG GOPROXY
ARG GOSUMDB
RUN go mod download

ADD cmd cmd/
ADD internal internal/
ADD public public/
COPY tools tools/
RUN /usr/local/go/bin/go run ./tools/genstatic.go public public

RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on taskset -c 1 /usr/local/go/bin/go build -a -o q3 ./cmd/q3

FROM alpine:3 as quake-n-bake

RUN apk add --no-cache git gcc make libc-dev
RUN git clone https://github.com/ioquake/ioq3
RUN cd /ioq3 && make BUILD_MISSIONPACK=0 BUILD_BASEGAME=0 BUILD_CLIENT=0 BUILD_SERVER=1 BUILD_GAME_SO=0 BUILD_GAME_QVM=0 BUILD_RENDERER_OPENGL2=0 BUILD_STANDALONE=1
# Note: the find command is used to find the ioq3ded binary, which the name of can vary depending on the build architecture.
# Unfortunately with the previous command the binary for arm64 was not found as it was looking for aarch64 instead of arm64 due 
# to the makefile setup in ioq3 repository.
RUN find /ioq3/build/ -type f -name "ioq3ded*" -exec cp {} /usr/local/bin/ioq3ded \;

FROM alpine:3

COPY --from=builder /workspace/q3 /usr/local/bin
COPY --from=quake-n-bake /usr/local/bin/ioq3ded /usr/local/bin
COPY --from=quake-n-bake /lib/ld-musl-*.so.1 /lib

ENTRYPOINT ["/usr/local/bin/q3"]
