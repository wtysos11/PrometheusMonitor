FROM ubuntu:18.04

RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

COPY test /usr/src/app/
RUN chmod 777 test
EXPOSE 8080
ENTRYPOINT ["./test"]
