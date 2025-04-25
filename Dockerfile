FROM golang AS builder

WORKDIR /src/app

COPY go.* *.go ./

RUN GOOS=linux GOARCH=386 go build -v -ldflags="-s -w" -o dvr ./

# -----

FROM ubuntu:latest

ENV DEBIAN_FRONTEND=noninteractive
ENV CONFIG_DIR=/config
ENV APP_DIR=/opt
ENV REC_DIR=/rec
ENV TZ=Etc/UTC

RUN apt update -y
RUN apt upgrade -y
RUN  apt install -y --no-install-recommends dvb-tools
RUN  mkdir ${CONFIG_DIR}
RUN  apt autoremove -y
RUN  apt purge -y 

COPY --from=builder /src/app/dvr ${APP_DIR}

WORKDIR ${APP_DIR}

ENTRYPOINT [ "./dvr" ]
