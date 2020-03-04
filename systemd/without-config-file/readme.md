systemd service file for Azure Storage fuse without config file.

# Step to install this to systemd
1. Download blobfuse.service and put it into /etc/systemd/system
2. Edit the file, changing environment values in Service block.
3. Run command to reload service config files: `systemctl daemon-reload`
4. Start service: `systemctl start blobfuse.service`
5. (Optional) Make the service starting with system: `systemctl enable blobfuse.service`
