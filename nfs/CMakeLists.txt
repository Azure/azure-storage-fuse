cmake_minimum_required(VERSION 3.7)

project(aznfsclient
        DESCRIPTION "FUSE driver for mounting Blob NFS shares"
        LANGUAGES CXX
        VERSION 0.1.0)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED True)

#
# Add options here.
#
option(ENABLE_TESTS "Build and run test programs" OFF)
option(ENABLE_DOCUMENTATION "Build Documentation" OFF)
option(ENABLE_NON_AZURE_NFS "Enable support for general NFS servers" ON)
option(ENABLE_CHATTY "Enable super verbose logs" OFF)
option(ENABLE_TCMALLOC "Use tcmalloc for malloc/free/new/delete" ON)

if(ENABLE_NON_AZURE_NFS)
  add_definitions(-DENABLE_NON_AZURE_NFS)
endif()

if(ENABLE_CHATTY)
  add_definitions(-DENABLE_CHATTY)
endif()

set(INSTALL_BIN_DIR "${CMAKE_INSTALL_PREFIX}/bin" CACHE PATH "Installation directory for binaries")
set(CMAKE_MODULE_PATH "${CMAKE_CURRENT_SOURCE_DIR}/CMake" ${CMAKE_MODULE_PATH})

#
# We add the following submodule(s), update them as needed, before starting.
# git submodule add https://github.com/linuxsmiths/libnfs extern/libnfs
# git submodule add https://github.com/jbeder/yaml-cpp.git extern/yaml-cpp
# git submodule add https://github.com/gabime/spdlog.git extern/spdlog
#
find_package(Git QUIET)
if(GIT_FOUND AND EXISTS "${PROJECT_SOURCE_DIR}/../.git")
    option(GIT_SUBMODULE "Check submodules during build" ON)
    if(GIT_SUBMODULE)
        message(STATUS "Submodule update")
        execute_process(COMMAND ${GIT_EXECUTABLE} submodule update --init --recursive
                        WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}
                        RESULT_VARIABLE GIT_SUBMOD_RESULT)
        if(NOT GIT_SUBMOD_RESULT EQUAL "0")
            message(FATAL_ERROR "git submodule update --init --recursive failed with ${GIT_SUBMOD_RESULT}, please checkout submodules")
        endif()
    endif()
endif()

if(NOT EXISTS "${PROJECT_SOURCE_DIR}/extern/libnfs/CMakeLists.txt")
    message(FATAL_ERROR "The libnfs submodule was not downloaded! GIT_SUBMODULE was turned off or failed. Please update submodules and try again.")
endif()

if(NOT EXISTS "${PROJECT_SOURCE_DIR}/extern/yaml-cpp/CMakeLists.txt")
    message(FATAL_ERROR "The yaml-cpp submodule was not downloaded! GIT_SUBMODULE was turned off or failed. Please update submodules and try again.")
endif()

if(NOT EXISTS "${PROJECT_SOURCE_DIR}/extern/spdlog/CMakeLists.txt")
    message(FATAL_ERROR "The spdlog submodule was not downloaded! GIT_SUBMODULE was turned off or failed. Please update submodules and try again.")
endif()

#
# Find fuse3 and if not found try to install.
#
find_package(fuse3 QUIET)
if(NOT fuse3_FOUND)
    message(STATUS "fuse3 not found, trying to install libfuse3-dev!")
    execute_process(COMMAND sudo apt install -y libfuse3-dev
		    RESULT_VARIABLE fuse3_INSTALL_RESULT)
    if(NOT fuse3_INSTALL_RESULT EQUAL "0")
        message(FATAL_ERROR "apt install libfuse3-dev failed with ${fuse3_INSTALL_RESULT}, try installing libfuse3-dev manually and then run cmake again")
    else()
	# Call once more to ensure above install completed fine and also
	# will set fuse3_INCLUDE_DIR and fuse3_LIBRARY variables.
        find_package(fuse3 REQUIRED)
    endif()
else()
    message(STATUS "Using fuse3 lib ${fuse3_LIBRARY}")
    message(STATUS "Using fuse3 include dir ${fuse3_INCLUDE_DIR}")
endif()

#
# We need GnuTLS for secure libnfs transport.
#
find_package(GnuTLS "3.4.6" QUIET)
if(NOT GNUTLS_FOUND)
    message(STATUS "GnuTLS not found, trying to install gnutls-dev!")
    execute_process(COMMAND sudo apt install -y gnutls-dev
                    RESULT_VARIABLE GNUTLS_INSTALL_RESULT)
    if(NOT GNUTLS_INSTALL_RESULT EQUAL "0")
        message(FATAL_ERROR "apt install gnutls-dev failed with ${GNUTLS_INSTALL_RESULT}, try installing gnutls-dev manually and then run cmake again")
    else()
        find_package(GnuTLS "3.4.6" REQUIRED)
    endif()
endif()

#
# Find tcmalloc and if not found try to install.
#
if(ENABLE_TCMALLOC)
find_package(tcmalloc QUIET)
if(NOT tcmalloc_FOUND)
    message(STATUS "tcmalloc not found, trying to install libgoogle-perftools-dev!")
    execute_process(COMMAND sudo apt install -y libgoogle-perftools-dev
            RESULT_VARIABLE tcmalloc_INSTALL_RESULT)
    if(NOT tcmalloc_INSTALL_RESULT EQUAL "0")
        message(FATAL_ERROR "apt install libgoogle-perftools-dev failed with ${tcmalloc_INSTALL_RESULT}, try installing libgoogle-perftools-dev manually and then run cmake again")
    else()
        # Call once more to ensure above install completed fine and also
        # will set tcmalloc_INCLUDE_DIR and tcmalloc_LIBRARY variables.
        find_package(tcmalloc REQUIRED)
    endif()
else()
    message(STATUS "Using tcmalloc lib ${tcmalloc_LIBRARY}")
    message(STATUS "Using tcmalloc include dir ${tcmalloc_INCLUDE_DIR}")
endif()
endif()

#
# Install zlib for crc32.
#
find_package(ZLIB QUIET)
if(NOT ZLIB_FOUND)
    message(STATUS "zlib not found, trying to install zlib1g-dev!")
    execute_process(COMMAND sudo apt install -y zlib1g-dev
		    RESULT_VARIABLE ZLIB_INSTALL_RESULT)
    if(NOT ZLIB_INSTALL_RESULT EQUAL "0")
        message(FATAL_ERROR "apt install zlib1g-dev failed with ${ZLIB_INSTALL_RESULT}, try installing zlib1g-dev manually and then run cmake again")
    else()
        # Call once more to ensure above install completed fine and also
        # will set ZLIB_INCLUDE_DIR and ZLIB_LIBRARY variables.
        find_package(ZLIB REQUIRED)
    endif()
else()
    message(STATUS "Using zlib lib ${ZLIB_LIBRARIES}")
    message(STATUS "Using zlib include dir ${ZLIB_INCLUDE_DIRS}")
endif()

# Build libnfs (with multithreading enabled).
set(ENABLE_MULTITHREADING ON)
set(ENABLE_EXAMPLES ON)
add_subdirectory(extern/libnfs)
add_subdirectory(extern/yaml-cpp)

# Build static library for spdlog.
set(SPDLOG_BUILD_SHARED OFF CACHE INTERNAL "Build spdlog shared library" FORCE)
set(BUILD_SHARED_LIBS OFF CACHE INTERNAL "Build shared library" FORCE)
add_subdirectory(extern/spdlog)

configure_file(inc/aznfsc_config.h.in inc/aznfsc_config.h)

# aznfsclient executable.
add_executable(${CMAKE_PROJECT_NAME}
               src/main.cpp
               src/nfs_client.cpp
               src/rpc_transport.cpp
               src/rpc_task.cpp
               src/rpc_readdir.cpp
               src/connection.cpp
               src/nfs_inode.cpp
               src/log.cpp
               src/file_cache.cpp
               src/readahead.cpp
               src/rpc_stats.cpp)

# All include directories.
target_include_directories(${CMAKE_PROJECT_NAME}
                           PRIVATE "${PROJECT_SOURCE_DIR}/inc"
                           PRIVATE "${PROJECT_BINARY_DIR}/inc"
                           PRIVATE "${fuse3_INCLUDE_DIR}"
                           PRIVATE "${ZLIB_INCLUDE_DIRS}"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/nfs"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/include"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/include/nfsc"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/mount"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/spdlog/include")

target_compile_options(${CMAKE_PROJECT_NAME}
                       PRIVATE -Wall
                       PRIVATE -Werror)

# All libraries.
target_link_libraries(${CMAKE_PROJECT_NAME}
                      ${fuse3_LIBRARY}
                      ${ZLIB_LIBRARIES}
                      pthread
                      nfs
                      yaml-cpp
                      spdlog)

if(ENABLE_TCMALLOC)
target_link_libraries(${CMAKE_PROJECT_NAME}
                      ${tcmalloc_LIBRARY})
endif()

install(TARGETS ${CMAKE_PROJECT_NAME})
