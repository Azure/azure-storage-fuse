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
               src/log.cpp)

# All include directories.
target_include_directories(${CMAKE_PROJECT_NAME}
                           PRIVATE "${PROJECT_SOURCE_DIR}/inc"
                           PRIVATE "${PROJECT_BINARY_DIR}/inc"
                           PRIVATE "${fuse3_INCLUDE_DIR}"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/include/nfsc"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/libnfs/mount"
                           PRIVATE "${PROJECT_SOURCE_DIR}/extern/spdlog/include")

target_compile_options(${CMAKE_PROJECT_NAME}
                       PRIVATE -Wall
                       PRIVATE -Werror)

# All libraries.
target_link_libraries(${CMAKE_PROJECT_NAME}
                      ${fuse3_LIBRARY}
                      pthread
                      nfs
                      yaml-cpp
                      spdlog)

install(TARGETS ${CMAKE_PROJECT_NAME})
