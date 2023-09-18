# Create container based on Ubuntu-22.04 Jammy Jellyfish image
FROM mcr.microsoft.com/mirror/docker/library/ubuntu:22.04 

# Create directory to hold samples
RUN mkdir -p /usr/share/blobfuse2

# Copy blobfuse2 binary to executable path
COPY ./blobfuse2 /usr/local/bin/
COPY ./config.yaml /usr/share/blobfuse2/

# Install fuse library
RUN \
	apt update && \
	apt-get install -y ca-certificates vim rsyslog && \
	apt-get install -y fuse3

RUN echo "user_allow_other" >> /etc/fuse.conf

# Create syslog filter files
COPY ./11-blobfuse2.conf /etc/rsyslog.d
COPY ./blobfuse2-logrotate /etc/logrotate.d/blobfuse2


# Create mount directory structure
RUN \
	mkdir -p /mnt/blobfuse_mnt && \
	mkdir -p /tmp/blobfuse_temp && \
	chmod 777 /mnt/blobfuse_mnt && \
	chmod 777 /tmp/blobfuse_temp


# Create the mount script and set it to entry point once container start
RUN \
	echo "/sbin/rsyslogd" > /usr/share/blobfuse2/blobfuse2-mount.sh && \
	echo "blobfuse2 mount /mnt/blobfuse_mnt --config-file=/usr/share/blobfuse2/config.yaml  --ignore-open-flags --foreground=true" >> /usr/share/blobfuse2/blobfuse2-mount.sh && \
	echo "blobfuse2 unmount all" > /usr/share/blobfuse2/blobfuse2-umount.sh && \
	chmod 777 /usr/share/blobfuse2/blobfuse2-mount.sh && \
	chmod 777 /usr/share/blobfuse2/blobfuse2-umount.sh && \
	ln -s  /usr/share/blobfuse2/blobfuse2-mount.sh /usr/local/bin/fuse && \
	ln -s  /usr/share/blobfuse2/blobfuse2-umount.sh /usr/local/bin/unfuse 


ENTRYPOINT ["bash", "fuse"]



