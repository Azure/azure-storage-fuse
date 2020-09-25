#!/bin/bash
#   blobfuse RPM SPEC generator and packager
#   Usage:
#       $ ./generaterpm.sh [-srcdir source_binaries_dir] [-distrover distro_version]
#
#   The above command will build an RPM package with the specified version
#   building the files in source_binaries_dir and packaging the blobfuse binary.
#   The RPM package will install the binaries in buildroot directory on
#   RPM installation
#   Note: By Nara V. These rpm commands are based on the command list from 
#          http://ftp.rpm.org/max-rpm/ch-rpm-b-command.html

while [ $# -gt 0 ]
do
     case "$1" in
        -srcdir) srcdir="$2"; shift;;  
        -distrover) distrover="$2"; shift;;
        --) shift; break;;
        -*)
                echo >&2 \
                "Usage: $0 [-srcdir source_directory] [-distrover distro_version]"
                exit 1;;
        *)  break;;     # terminate while loop
    esac
    shift
done
echo "Source dir: " ${srcdir};
echo "Linux distro and version:" ${distrover};

# Read the CMakeLists.txt to get the vrsion number
while read -r line;
do
        # reading each line 
       # if [[$line =~ "CPACK_PACKAGE_VERSION_MAJOR" ]]
       # then    
       #     # ver_major = grep -o '[0-9]*' "${line}"
       #     echo $line
       # fi 
       case "$line" in 
                 *CPACK_PACKAGE_VERSION_MAJOR*)
                 ver_major="${line//[!0-9]/}"
                ;;
                 *CPACK_PACKAGE_VERSION_MINOR*)
                 ver_minor="${line//[!0-9]/}"
                ;;
                 *CPACK_PACKAGE_VERSION_PATCH*)
                 ver_patch="${line//[!0-9]/}"
                ;;
                 *CPACK_PACKAGE_VERSION_RELEASE*)
                 ver_release="${line//[!0-9]/}"
                ;;
       esac
        
done < "../CMakeLists.txt"

# check if major version is set
if [ -z "${ver_major}" ]; then
        echo Error: Check the CMakeLists.txt. It should set the CPACK_PACKAGE_VERSION_MAJOR
        exit 1
fi

# check if minor version is set
if [ -z "${ver_minor}" ]; then
        echo Error: Check the CMakeLists.txt. It should set the CPACK_PACKAGE_VERSION_MINOR
        exit 1
fi

# check if patch version is set
if [ -z "${ver_patch}" ]; then
        echo Error: Check the CMakeLists.txt. It should set the CPACK_PACKAGE_VERSION_PATCH
        exit 1
fi

# check if release version is set
if [ -z "${ver_release}" ]; then
        echo Error: Check the CMakeLists.txt. It should set the CPACK_PACKAGE_VERSION_RELEASE
        exit 1
fi

version=${ver_major}.${ver_minor}.${ver_patch}

echo "Version: " ${version};

# check if the rpmcontent directory exists
if [ ! -d "${srcdir}" ]; then
        echo Error: Directory ${srcdir} does not exists
        exit 1
fi

# check the first parameter expected (version) has been passed
if [ -z "${version}" ]; then
        echo Error: Build version number is not set. Check your CMAkeLists for MAJOR, MINOR, PATCH and RELEASE numerical values
        exit 1
fi


# These are needed directories for the rpmbuild package
directories=( ~/rpmbuild ~/rpmbuild/SOURCES ~/rpmbuild/SPECS)

echo "Building RPM package for blobfuse version" $version;

origdir=`pwd`
echo "From: " ${origdir};

# prepare tree
for dir in ${directories[*]}
do
        mkdir -p $dir
        echo "created dir" $dir
done

RPM_BUILD_ROOT=~/rpmbuild

cd ${srcdir}
echo "Creating Tar from: "${srcdir};

mkdir -p blobfuse-${version}
cp blobfuse blobfuse-${version}/
cp ../README.md blobfuse-${version}/
cp ../LICENSE blobfuse-${version}/

tar -cvjSf blobfuse-${version}-${distrover}.tar.bz2 blobfuse-${version}
echo "Copying Tar to: ~/rpmbuild/SOURCES";
mv blobfuse-${version}-${distrover}.tar.bz2 ~/rpmbuild/SOURCES
cd -

# prepare files for rpmbuild
# setting the buildroot below has no effect. buildroot is always appending the %{_arch} at the end

cat <<EOF >~/rpmbuild/.rpmmacros
%{_topdir}   %(~/rpmbuild)
%{_tmppath} %{_topdir}/tmp
%{buildroot} %{_topdir}/BUILDROOT/%{name}-%{version}-%{release}
%{_bindir}   /usr/bin

EOF

# disable check-buildroot (normally /usr/lib/rpm/check-buildroot) with:%define __arch_install_post %{nil}
# also disable package debug and check_spec to save time

cat <<EOF > ~/rpmbuild/SPECS/blobfuse.spec
%define __arch_install_post %{nil}
%define debug_package %{nil}
%define __spec_install_post %{nil}
Summary:   FUSE adapter - Azure Storage Blobs
Name: blobfuse
Version: $version
Release: $ver_release%{?dist}
License: MIT.
Group: Applications/Tools
SOURCE0 : blobfuse-${version}-${distrover}.tar.bz2
URL: http://github.com/Azure/azure-storage-fuse/


BuildArch: x86_64
BuildRoot: %(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-%{_arch}-XXXX)
BuildRequires:    boost-thread
BuildRequires:    boost-system
BuildRequires:    boost-filesystem

Requires: fuse >= 2.2.7

%description
FUSE adapter - Azure Storage Blob file mount adapter using the fuse library

%prep

%setup -q

%build
# make


%install
# though this has the name install this is run while building the rpm
# it is failing in the below command with some -p option
mkdir -p %{buildroot}/%{_bindir}
install -m 0755 %{name} %{buildroot}/%{_bindir}/%{name}
#install -p -m 755 blobfuse $RPM_BUILD_ROOT/BUILDROOT/blobfuse-$version-%{release}-%{_arch}

%clean
rm -rf %{buildroot}

%files
%defattr(555,root,root,555)
/usr/bin/blobfuse

%changelog
* $(date +"%a %b %d %Y") Blobfuse Dev blobfusedev@microsoft.com> ${version}
- Building RPM package for Blobfuse using rpmbuilder

EOF

# run rpmbuild from its folder
cd ~/rpmbuild/SPECS
rpmbuild --target ./ --nodeps -ba blobfuse.spec
cd -

# copy RPM output to the local directory and add the distro name as part of it.

cp ~/rpmbuild/RPMS/x86_64/blobfuse-${version}-*x86_64.rpm .
mv blobfuse-${version}-*x86_64.rpm blobfuse-${version}-${distrover}.rpm
