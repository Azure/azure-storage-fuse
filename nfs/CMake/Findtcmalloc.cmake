# Findtcmalloc
# Finds tcmalloc includes and library
#
# Result variables:
#  tcmalloc_FOUND       - tcmalloc library was found
#  tcmalloc_INCLUDE_DIR - tcmalloc include directory
#  tcmalloc_LIBRARY     - Library needed to use tcmalloc

# check if already in cache, be silent
if (tcmalloc_INCLUDE_DIR AND tcmalloc_LIBRARY)
    set(tcmalloc_FIND_QUIETLY TRUE)
endif()

# find includes
find_path(tcmalloc_INCLUDE_DIR gperftools/tcmalloc.h
    /usr/local/include
    /usr/include)

# find lib
find_library(tcmalloc_LIBRARY
    NAMES tcmalloc libtcmalloc
    PATHS /lib64 /lib /usr/lib64 /usr/lib /usr/local/lib64 /usr/local/lib /usr/lib/x86_64-linux-gnu)

include("FindPackageHandleStandardArgs")
find_package_handle_standard_args(tcmalloc DEFAULT_MSG
    tcmalloc_INCLUDE_DIR
    tcmalloc_LIBRARY)

mark_as_advanced(tcmalloc_INCLUDE_DIR tcmalloc_LIBRARY)
