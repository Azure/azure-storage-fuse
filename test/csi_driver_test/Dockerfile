FROM k8s.gcr.io/build-image/debian-base:bullseye-v1.1.0

RUN apt update && apt-mark unhold libcap2
RUN clean-install ca-certificates uuid-dev util-linux mount udev wget fuse

# Before building this Dockerfile ensure blobfuse2.deb is in the current directory.
COPY blobfuse2.deb /tmp/blobfuse2.deb
RUN dpkg -i /tmp/blobfuse2.deb