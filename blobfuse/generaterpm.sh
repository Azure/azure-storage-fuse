#!/bin/bash
#   Blobfuse RPM SPEC generator and packager
#   Usage:
#       $ ./rpmbuilder.sh [-ver apps_version] [-srcdir source_binaries_dir] [-dstdir destination_dir]
#
#   The above command will build an RPM package with the specified version
#   building the files in source_binaries_dir and packaging the blobfuse binary.
#   If no source binaries folder is specified then ~/dev is used.
#   The RPM package will install the binaries in destination_dir on
#   RPM installation

mkdir ./rpmbuild;
srcdir="`echo ~`./rpmbuild.";
dstdir="./";

cp ./README.md ./rpmbuild;
cp ./LICENSE ./rpmbuild;
cp ./build/blobfuse ./rpmbuild;
tar -cvjSf blobfuse-$2-.tar.bz2 folder
mkdir ./rpmbuild/SOURCES;
mkdir ./rpmbuild/SPECS;

while [ $# -gt 0 ]
do
     case "$1" in
        -srcdir) srcdir="$2"; shift;;
        -ver) version="$2"; shift;;
        -sdkver) sdkver="$2"; shift;;
        -dstdir) dstdir="$2"; shift;;        
        -distrover) distrover="$2"; shift;;
        --) shift; break;;
        -*)
                echo >&2 \
                "Usage: $0 [-ver version] [-sdkver sdkver] [-srcdir source_directory] [-dstdir destination_directory]  [-distrover distro_version]"
                exit 1;;
        *)  break;;     # terminate while loop
    esac
    shift
done

echo "Version: " ${version};
echo "EOS SDK Version: " ${sdkver};
echo "Source dir: " ${srcdir};
echo "Destination dir: " ${dstdir};
echo "Linux distro and version:" ${distrover};

# check if the rpmcontent directory exists
if [ ! -d "${srcdir}" ]; then
        echo Error: Directory ${srcdir} does not exists
        exit 1
fi

# check the first parameter expected (version) has been passed
if [ -z "${version}" ]; then
        echo Error: Expected version parameter. Ex. for version 1.0 use: ./rpmbuilder.sh -ver 1.0
        exit 1
fi

# These are needed directories for the rpmbuild package
directories=( ./rpmbuild ./rpmbuild/SOURCES ./rpmbuild/SPECS )

echo "Building RPM package for version" $version;

origdir=`pwd`
echo "From: " ${origdir};

# prepare tree
for dir in ${directories[*]}
do
        mkdir -p $dir
done

cd ${srcdir}
echo "Creating Tar from: "${srcdir};
tar -zcvf blobfuse-${version}.tar.gz RouterAgent http-parser
echo "Copying Tar to: ~/rpmbuild/SOURCES";
cp TerAgent-${version}.tar.gz ~/rpmbuild/SOURCES
cd -

# prepare files for rpmbuild
cat <<EOF >~/rpmbuild/.rpmmacros
%_topdir   %(echo `pwd`)
%_tmppath  %{_topdir}/tmp
%_bindir   ${dstdir}
EOF
cat <<EOF > ~/rpmbuild/SPECS/blobfuse.spec
%define        __spec_install_post %{nil}
%define          debug_package %{nil}
%define        __os_install_post %{_dbpath}/brp-compress

Summary: :        FUSE adapter - Azure Storage Blobs
Name: Blobfuse
Version: $version
Release: $sdkver
License: MIT.
Group: Applications/Tools
SOURCE0 : %{name}-%{version}.tar.gz
URL: http://github.com/Azure/azure-storage-fuse/

BuildRoot: %{_tmppath}/%{name}-%{version}-%{release}-root

Requires: fuse >= 2.2.7


%description
%{summary}

%prep
# %setup -q
rm -rf blobfuse-$version
mkdir blobfuse-$version
tar xzvf %_sourcedir/blobfuse-$version.tar.gz -C ./TerAgent-$version

%build
cd TerAgent-$version/RouterAgent/TransitEdgeRouter/Arista
make clean
make

%install
rm -rf %{buildroot}
mkdir -p  %{buildroot}/usr/local/bin/ter
mkdir -p  %{buildroot}/usr/lib/SysdbMountProfiles

# Copy in builddir (this is neeed bcs we don't have a Makefile in the root)
# ideally we should have a Makefile in the root and do make install there.
cp -a TerAgent-$version/RouterAgent/TransitEdgeRouter/Arista/TerAgent %{buildroot}/usr/local/bin/ter
cp -a TerAgent-$version/RouterAgent/TransitEdgeRouter/Arista/SysdbMountProfiles/TerAgent %{buildroot}/usr/lib/SysdbMountProfiles

%clean

%files
%defattr(555,root,root,555)
/usr/local/bin/ter/*
%attr(644,root,root) /usr/lib/SysdbMountProfiles/*

%changelog
* Thu Aug 01 2018 Azure WAN Team <wandp@microsoft.com>
- Building RPM package for TerAgent using rpmbuilder

EOF

# run rpmbuild from its folder
cd ~/rpmbuild/SPECS
rpmbuild --target i386 --nodeps -ba TerAgent.spec
cd -

# copy RPM output to the local directory
cp ~/rpmbuild/RPMS/i386/TerAgent-${version}-${sdkver}.i386.rpm .
