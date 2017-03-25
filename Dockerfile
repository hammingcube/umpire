FROM golang:1.7

ENV SRCDIR /go/src/github.com/maddyonline

COPY . ${SRCDIR}/umpire

COPY files/clean_dir/optcode-secrets ${SRCDIR}/optcode-secrets
COPY files/clean_dir/problemset ${SRCDIR}/problemset

RUN cd ${SRCDIR}/umpire && go get -v ./...
RUN cd ${SRCDIR}/umpire && umpire update ../problemset

WORKDIR ${SRCDIR}/umpire
EXPOSE 1323
ENTRYPOINT ["/go/bin/umpire-server"]
