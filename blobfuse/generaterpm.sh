#!/bin/bash
#   Blobfuse RPM SPEC generator and packager
#   Usage:
#       $ ./rpmbuilder.sh [-ver apps_version] [-srcdir source_binaries_dir] [-distrover distro_version]
#
#   The above command will build an RPM package with the specified version
#   building the files in source_binaries_dir and packaging the blobfuse binary.
#   If no source binaries folder is specified then ~/dev is used.
#   The RPM package will install the binaries in destination_dir on
#   RPM installation


while [ $# -gt 0 ]
do
     case "$1" in
        -srcdir) srcdir="$2"; shift;;
        -ver) version="$2"; shift;;      
        -distrover) distrover="$2"; shift;;
        --) shift; break;;
        -*)
                echo >&2 \
                "Usage: $0 [-ver version] [-srcdir source_directory] [-distrover distro_version]"
                exit 1;;
        *)  break;;     # terminate while loop
    esac
    shift
done

echo "Version: " ${version};
echo "Source dir: " ${srcdir};
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
cp README.md blobfuse-${version}/
cp LICENSE blobfuse-${version}/

tar -cvjSf blobfuse-${version}-${distrover}.tar.bz2 blobfuse-${version}
echo "Copying Tar to: ~/rpmbuild/SOURCES";
mv blobfuse-${version}-${distrover}.tar.bz2 ~/rpmbuild/SOURCES
cd -


# prepare files for rpmbuild
cat <<EOF >~/rpmbuild/.rpmmacros
%_topdir   %(~/rpmbuild)
%_tmppath  %{_topdir}/tmp
%_bindir    BUILDROOT
EOF
cat <<EOF > ~/rpmbuild/SPECS/blobfuse.spec

Summary:   FUSE adapter - Azure Storage Blobs
Name: blobfuse
Version: $version
Release: 1
License: MIT.
Group: Applications/Tools
SOURCE0 : blobfuse-${version}-${distrover}.tar.bz2
URL: http://github.com/Azure/azure-storage-fuse/

BuildRoot: %(mktemp -ud %{_tmppath}/%{name}-%{version}-%{release}-XXXXXX)
BuildRequires:    boost-thread
BuildRequires:    boost-system
BuildRequires:    boost-filesystem

Requires: fuse >= 2.2.7

%description
%{Summary}

%prep

%setup -q

%build
# make


%install
# though this has the name install this is run while building the rpm
#rm -rf $RPM_BUILD_ROOT
#mkdir -p $RPM_BUILD_ROOT/BUILD/blobfuse-${version}
#install -p -m 755 blobfuse $RPM_BUILD_ROOT/BUILDROOT/blobfuse-$version-%{release}-%{_arch}

%clean
rm -rf $RPM_BUILD_ROOT

%files
%defattr(555,root,root,555)
/usr/bin/blobfuse

%changelog
* $(date) Blobfuse Dev blobfusedev@microsoft.com> ${version}
- Building RPM package for Blobfuse using rpmbuilder

EOF

# run rpmbuild from its folder
cd ~/rpmbuild/SPECS
rpmbuild --target ./ --nodeps -ba blobfuse.spec
cd -

# copy RPM output to the local directory
cp ~/rpmbuild/blobfuse-${version}-${distrover}.rpm .
