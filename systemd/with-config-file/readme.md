systemd service file for Azure Storage fuse with config file.

# Step to install this to systemd
1. Prepare the config file. Name of the container should be specified by `containerName` in config file. See [this](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-how-to-mount-container-linux) for details.
2. Download blobfuse.service and put it into /etc/systemd/system
3. Edit the file, changing environment values in Service block.
4. Run command to reload service config files: `systemctl daemon-reload`
5. Start service: `systemctl start blobfuse.service`
6. (Optional) Make the service starting with system: `systemctl enable blobfuse.service`
