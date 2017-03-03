FROM ubuntu

ENV GOPATH /go
ENV SRCDIR /go/src/github.com/maddyonline

RUN mkdir -p $SRCDIR/optcode-secrets
RUN mkdir -p $SRCDIR/umpire


COPY files/optimal-code-admin.json $SRCDIR/optcode-secrets/optimal-code-admin.json
COPY files/umpire-server $SRCDIR/umpire/umpire-server

WORKDIR $SRCDIR/umpire

EXPOSE 1323


