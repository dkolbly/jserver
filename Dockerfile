FROM golang:1.3.3-wheezy

RUN apt-get update && apt-get install -y cmake pkg-config build-essential
RUN go get -d github.com/libgit2/git2go
RUN cd /go/src/github.com/libgit2/git2go ; git submodule update --init ; make install

MAINTAINER Donovan Kolbly <donovan@rscheme.org>

ADD . /go/src/github.com/dkolbly/jserver

RUN go get github.com/dkolbly/jserver
