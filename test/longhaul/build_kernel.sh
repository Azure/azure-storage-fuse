
# Install esseentials
sudo apt-get install git fakeroot build-essential ncurses-dev xz-utils libssl-dev bc flex libelf-dev bison -y

# Get to mount path
cd $1
version=$2

# Download kernel tar ball
wget https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$version.tar.xz

# Extract tarball
tar -xvf linux-$version.tar.xz

# Create default config
cd linux-$version
make defconfig

# build the kernel
make
