FROM ubuntu:18.04

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY test /usr/src/app/
EXPOSE 8080
ENTRYPOINT ["./test"]
