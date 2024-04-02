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


#
# We add the following submodule(s), update them as needed, before starting.
# git submodule add https://github.com/linuxsmiths/libnfs extern/libnfs
# git submodule add https://github.com/jbeder/yaml-cpp.git extern/yaml-cpp
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

# Build libnfs.
add_subdirectory(extern/libnfs)
add_subdirectory(extern/yaml-cpp)

configure_file(inc/aznfsc_config.h.in inc/aznfsc_config.h)

# aznfsclient executable.
add_executable(${CMAKE_PROJECT_NAME}
               src/main.cpp)

# All include directories.
target_include_directories(${CMAKE_PROJECT_NAME}
                           PRIVATE ${PROJECT_SOURCE_DIR}/inc
                           PRIVATE ${PROJECT_BINARY_DIR}/inc
                           PRIVATE ${PROJECT_SOURCE_DIR}/extern/libnfs/include/nfsc
                           PRIVATE ${PROJECT_SOURCE_DIR}/extern/libnfs/mount)

target_compile_options(${CMAKE_PROJECT_NAME}
                       PRIVATE -Wall
                       PRIVATE -Werror)

# All libraries.
target_link_libraries(${CMAKE_PROJECT_NAME}
                      fuse3
                      pthread
                      nfs
                      yaml-cpp)

install(TARGETS ${CMAKE_PROJECT_NAME})
