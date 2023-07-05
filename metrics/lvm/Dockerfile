FROM alpine

RUN apk add --no-cache lvm2

WORKDIR /

COPY lvm-exporter /lvm-exporter
ENTRYPOINT ["/lvm-exporter"]