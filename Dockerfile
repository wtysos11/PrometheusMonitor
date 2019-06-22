FROM ubuntu:18.04

COPY test ./
EXPOSE 8080
ENTRYPOINT ["./test"]

