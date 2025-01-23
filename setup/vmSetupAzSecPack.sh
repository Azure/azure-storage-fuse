# Script to setup Azsecpack on Ubuntu VM as per recent SFI guidelines
!/bin/bash

# Install Azure CLI
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash

# Update package lists
sudo apt-get update -y

# Install required packages
sudo apt-get install apt-transport-https ca-certificates curl gnupg lsb-release -y

# Create directory for Microsoft GPG key
sudo mkdir -p /etc/apt/keyrings

# Download and install Microsoft GPG key
curl -sLS https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor | sudo tee /etc/apt/keyrings/microsoft.gpg > /dev/null

# Set permissions for the GPG key
sudo chmod go+r /etc/apt/keyrings/microsoft.gpg

# Get the distribution codename
AZ_DIST=$(lsb_release -cs)

# Add Azure CLI repository to sources list
echo "Types: deb
URIs: https://packages.microsoft.com/repos/azure-cli/
Suites: ${AZ_DIST}
Components: main
Architectures: $(dpkg --print-architecture)
Signed-by: /etc/apt/keyrings/microsoft.gpg" | sudo tee /etc/apt/sources.list.d/azure-cli.sources

# Install Azure CLI
sudo apt-get install azure-cli -y

# Update package lists again
sudo apt-get update

# Install Azure CLI again to ensure it's up to date
sudo apt-get install azure-cli -y

# Remove unnecessary packages
sudo apt autoremove -y

# Upgrade Azure CLI to the latest version
az upgrade -y

#-------------------------------------------------------------------------------------------------------

# Log in to Azure
# You will get a pop-up here select your account and login
echo "You will get a pop-up here select your account and login"
echo "PLEASE NOTE: After az login you should select the Subscription you are on and enter that Subscription ID : 
\\n For Example: XCLient 116 is shown in the list of subscriptions, you should then enter 116"
az login --tenant 72f988bf-86f1-41af-91ab-2d7cd011db47

# Extracting VM name from hostname
vm_name=$(hostname)

# Extracting resource group name from Azure Instance Metadata Service
resource_group=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance?api-version=2021-02-01" -s | jq -r '.compute.resourceGroupName')

# Check if VM name and resource group are not empty
if [ -z "$vm_name" ] || [ -z "$resource_group" ]; then
    echo "Failed to retrieve VM name or resource group. You will have to manually insert these values in the upcoming commands"
    exit 1
fi

# Install Azure Monitor Linux Agent extension
# az vm extension set -n AzureMonitorLinuxAgent --publisher Microsoft.Azure.Monitor --version 1.0 --vm-name <vm-name> --resource-group <rg-name> --enable-auto-upgrade true --settings '{"GCS_AUTO_CONFIG": true}'
az vm extension set -n AzureMonitorLinuxAgent --publisher Microsoft.Azure.Monitor --version 1.0 --vm-name $vm_name --resource-group $resource_group --enable-auto-upgrade true --settings '{"GCS_AUTO_CONFIG": true}'

# Install Azure Security Linux Agent extension
# az vm extension set -n AzureSecurityLinuxAgent --publisher Microsoft.Azure.Security.Monitoring --version 2.0 --vm-name <vm-name> --resource-group <rg-name> --enable-auto-upgrade true --settings '{"enableGenevaUpload":true,"enableAutoConfig":true}'
az vm extension set -n AzureSecurityLinuxAgent --publisher Microsoft.Azure.Security.Monitoring --version 2.0 --vm-name $vm_name --resource-group $resource_group --enable-auto-upgrade true --settings '{"enableGenevaUpload":true,"enableAutoConfig":true}'

# Check the status of Azure Security Pack
status_output=$(sudo /usr/local/bin/azsecd status)

# Check if AutoConfig is enabled
if echo "$status_output" | grep -Pzo "AutoConfig:\n\s+Enabled\(true\)" > /dev/null; then
    autoconfig_enabled="true"
else
    autoconfig_enabled="false"
fi
# Check if AzSecPack is present in ResourceTags
azsecpack_present=$(echo "$status_output" | grep -q 'AzSecPack:\s*IsPresent(true)' && echo "true" || echo "false")

if [ "$autoconfig_enabled" = "true" ]; then
    echo "AutoConfig is enabled."
else
    echo "AutoConfig is not enabled. Please manually check if any installation step has failed."
fi

if [ "$azsecpack_present" = "true" ]; then
    echo "AzSecPack is present in ResourceTags."
else
    echo "AzSecPack is not present in ResourceTags.Please manually check if any installation step has failed."
fi

echo "Please check the status of Azure Security Pack by running 'sudo /usr/local/bin/azsecd status'"
echo "Installation of Azure Security Pack is complete.If you found any errors please manually check the installation steps."
#-------------------------------------------------------------------------------------------------------
# Define the command you want to run
COMMAND="az vm assess-patches --resource-group $resource_group --name $vm_name"

# Initialize variables
attempt=0
start_time=$(date +%s)

# Loop until the command is successful
while true; do
  attempt=$((attempt + 1))
  echo "Attempt $attempt: Trying to run the command..."

  # Run the command
  $COMMAND

  # Check if the command was successful
  if [ $? -eq 0 ]; then
    echo "Command executed successfully on attempt $attempt."
    break
  else
    echo "Command failed. Retrying..."
  fi

  # Optional: Add a sleep interval between attempts
  sleep 1
done

# Measure the end time
end_time=$(date +%s)
elapsed_time=$((end_time - start_time))

# Check for pending updates, assess and install patches
#az vm assess-patches --resource-group <rg-name> --name <vm-name>
az vm install-patches --resource-group $resource_group --name $vm_name --maximum-duration PT2H --reboot-setting IfRequired --classifications-to-include-linux Critical Security