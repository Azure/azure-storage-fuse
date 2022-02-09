systemd service file for blobfuse2

# Step to install this to systemd
1. If you are existing blobfuse user follow the MIGRATION.md file on how to convert blobfuse config and cli parameters to blobfuse2 compliant config
2. Prepare the config file. Name of the container should be specified by `azstorage:container` in config file or AZURE_STORAGE_ACCOUNT_CONTAINER environment variable. 
3. Download blobfuse2.service and put it into /etc/systemd/system
4. Edit the file, changing environment values in Service block.
5. Run command to reload service config files: `systemctl daemon-reload`
6. Start service: `systemctl start blobfuse2.service`
7. Make the service starting with system: `systemctl enable blobfuse2.service`
8. Please Note that the example has the User AzureUser, please create a user called AzureUser or replace this with an existing user.
