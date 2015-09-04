FROM phusion/baseimage:0.9.17
MAINTAINER "André Martins <aanm90@gmail.com>"
COPY . /docker-collector
ENTRYPOINT ["/docker-collector/docker-collector-Linux-x86_64"]
