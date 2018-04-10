FROM golang:1.10

RUN apt-get update -y && apt-get install -y redis-server

ADD . /go/src/github.com/blocksports/block-sports-scheduler

WORKDIR /go/src/github.com/blocksports/block-sports-scheduler

EXPOSE 5000

RUN chmod +x ./docker/run.sh

CMD ./docker/run.sh
