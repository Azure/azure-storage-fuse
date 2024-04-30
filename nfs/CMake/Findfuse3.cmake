# Findfuse3
# Finds fuse3 includes and library
#
# Result variables:
#  fuse3_FOUND       - fuse3 library was found
#  fuse3_INCLUDE_DIR - fuse3 include directory
#  fuse3_LIBRARY     - Library needed to use fuse3

# check if already in cache, be silent
if (fuse3_INCLUDE_DIR AND fuse3_LIBRARY)
    set(fuse3_FIND_QUIETLY TRUE)
endif()

# find includes
find_path(fuse3_INCLUDE_DIR fuse3/fuse.h
	/usr/local/include
	/usr/include)

# find lib
find_library(fuse3_LIBRARY
        NAMES fuse3 libfuse3
        PATHS /lib64 /lib /usr/lib64 /usr/lib /usr/local/lib64 /usr/local/lib /usr/lib/x86_64-linux-gnu)

include("FindPackageHandleStandardArgs")
find_package_handle_standard_args(fuse3 DEFAULT_MSG
        fuse3_INCLUDE_DIR
        fuse3_LIBRARY)

mark_as_advanced(fuse3_INCLUDE_DIR fuse3_LIBRARY)
