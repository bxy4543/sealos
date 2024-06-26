FROM alpine:3.18.4
RUN apk add --no-cache lvm2 lvm2-extra util-linux device-mapper
RUN apk add --no-cache btrfs-progs xfsprogs xfsprogs-extra e2fsprogs e2fsprogs-extra
RUN apk add --no-cache ca-certificates libc6-compat

ARG DBUILD_DATE
ARG DBUILD_REPO_URL
ARG DBUILD_SITE_URL

WORKDIR /

COPY lvm_exporter /lvm_exporter
ENTRYPOINT ["/lvm_exporter"]