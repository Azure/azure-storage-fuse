if groups | grep "\<sudo\>" &> /dev/null; then
   sudo rm -rf /usr/share/blobfuse2/
   sudo mkdir /usr/share/blobfuse2
fi