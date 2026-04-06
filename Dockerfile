FROM golang:1.23 AS builder

WORKDIR /src/app

COPY go.* *.go ./

RUN GOOS=linux GOARCH=386 go build -v -ldflags="-s -w" -o dvr ./

# -----

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
ENV CONFIG_DIR=/config
ENV APP_DIR=/opt
ENV REC_DIR=/rec
ENV TZ=Etc/UTC

RUN apt-get update -y && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends dvb-tools && \
    mkdir ${CONFIG_DIR} && \
    apt-get autoremove -y && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/app/dvr ${APP_DIR}

WORKDIR ${APP_DIR}

ENTRYPOINT [ "./dvr" ]
