FROM golang:1.3.3-wheezy

MAINTAINER Donovan Kolbly <donovan@rscheme.org>

ADD . /go/src/github.com/dkolbly/jserver
RUN go install github.com/dkolbly/jserver
