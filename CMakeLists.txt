cmake_minimum_required(VERSION 2.8)

set (BLOBFUSE_HEADER
  blobfuse/blobfuse.h
  blobfuse/blob_client_cache.h
  #blobfuse/blobfuse_constants.h
  blobfuse/OAuthToken.h
  blobfuse/OAuthTokenCredentialManager.h
)

set (BLOBFUSE_SOURCE
#  blobfuse/blobfuse.cpp    # We don't include blobfuse.cpp because the file contents are directly #included in main.cpp.  This allows us to split main() and everything
#                             else into different files, without having to define each function in a header for only one consumer.  The only reason to do all this is so
#                             we can test functionality in blobfuse.cpp without including main() (which would break gtest).
  blobfuse/blobfuse_constants.cpp
  blobfuse/fileapis.cpp
  blobfuse/directoryapis.cpp
  blobfuse/utilities.cpp
  blobfuse/OAuthToken.cpp
  blobfuse/OAuthTokenCredentialManager.cpp
)

if(UNIX)
  project(blobfuse)
  find_package(Threads REQUIRED)
  find_package(CURL REQUIRED)
  find_package(GnuTLS REQUIRED)
  find_package(PkgConfig)
  find_package(Boost REQUIRED)
  set(Boost_USE_STATIC_LIBS ON)
  find_package(Boost COMPONENTS filesystem system REQUIRED)
  find_package(Boost COMPONENTS thread REQUIRED)
  add_definitions(-std=c++11)
  add_definitions(-D_FILE_OFFSET_BITS=64)
 # set(WARNING "-Wall -Wextra -Werror -pedantic -pedantic-errors")
  set(WARNING "-Wall -Wextra -Werror")
  set(CMAKE_CXX_FLAGS "${CMAKE_THREAD_LIBS_INIT} ${WARNING} ${CMAKE_CXX_FLAGS}")
  include_directories(${CMAKE_SOURCE_DIR}/blobfuse ${CMAKE_SOURCE_DIR}/cpplite/include ${CURL_INCLUDE_DIRS} ${GNUTLS_INCLUDE_DIR} ${Boost_INCLUDE_DIR} nlohmann-json)

  set(CMAKE_MACOSX_RPATH ON)

  add_executable(blobfuse ${BLOBFUSE_HEADER} ${BLOBFUSE_SOURCE} blobfuse/main.cpp)
  add_subdirectory(cpplite)
  add_dependencies (blobfuse azure-storage-lite)

  pkg_search_module(UUID REQUIRED uuid)
  target_link_libraries(blobfuse ${CURL_LIBRARIES} ${GNUTLS_LIBRARIES} ${CMAKE_THREAD_LIBS_INIT} ${UUID_LIBRARIES} ${Boost_FILESYSTEM_LIBRARY} ${Boost_SYSTEM_LIBRARY} ${Boost_THREAD_LIBRARY} azure-storage-lite fuse gcrypt)
  install(TARGETS blobfuse
    PERMISSIONS OWNER_EXECUTE OWNER_WRITE OWNER_READ GROUP_READ GROUP_EXECUTE WORLD_READ WORLD_EXECUTE
    DESTINATION bin)

  ## build blobcp
  execute_process (
    COMMAND bash -c "go build blobcp.go"
    WORKING_DIRECTORY ${CMAKE_SOURCE_DIR}/tools
    RESULT_VARIABLE exitStatus
  )

  ## if build worked, install blobcp
  if(NOT ${exitStatus})
	install(FILES ${CMAKE_SOURCE_DIR}/tools/blobcp
          PERMISSIONS OWNER_EXECUTE OWNER_WRITE OWNER_READ GROUP_READ GROUP_EXECUTE WORLD_READ WORLD_EXECUTE
          DESTINATION bin)
  endif()

  set(CPACK_GENERATOR "DEB")
  set(CPACK_DEBIAN_PACKAGE_MAINTAINER "Microsoft - Azure Storage")
  set(CPACK_DEBIAN_PACKAGE_DEPENDS "fuse")
  set(CPACK_DEBIAN_PACKAGE_DESCRIPTION "blobfuse 1.2.3 - FUSE adapter for Azure Blob Storage")
  set(VERSION "1.2.3")
  set(CPACK_PACKAGE_VERSION_MAJOR 1)
  set(CPACK_PACKAGE_VERSION_MINOR 2)  
  set(CPACK_PACKAGE_VERSION_PATCH 3)
  set(CPACK_PACKAGE_VERSION_RELEASE 1)
  set(CPACK_PACKAGE_NAME "blobfuse")
  set(CPACK_PACKAGE_VENDOR "Microsoft")
  include(CPack)
endif(UNIX)

if(INCLUDE_TESTS)
  # Download and build googletest at cmake time, as recommended by the googletest docs:
  # https://github.com/google/googletest/blob/master/googletest/README.md

  # Download and unpack googletest at configure time
  configure_file(test/CMakeLists.txt.in googletest-download/CMakeLists.txt)
  execute_process(COMMAND ${CMAKE_COMMAND} -G "${CMAKE_GENERATOR}" .
    RESULT_VARIABLE result
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}/googletest-download )
  if(result)
    message(FATAL_ERROR "CMake step for googletest failed: ${result}")
  endif()
  execute_process(COMMAND ${CMAKE_COMMAND} --build .
    RESULT_VARIABLE result
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}/googletest-download )
  if(result)
    message(FATAL_ERROR "Build step for googletest failed: ${result}")
  endif()

  # Prevent overriding the parent project's compiler/linker
  # settings on Windows
  set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)

  # Add googletest directly to our build. This defines
  # the gtest and gtest_main targets.
  add_subdirectory(${CMAKE_BINARY_DIR}/googletest-src
                   ${CMAKE_BINARY_DIR}/googletest-build
                   EXCLUDE_FROM_ALL)

  # The gtest/gtest_main targets carry header search path
  # dependencies automatically when using CMake 2.8.11 or
  # later. Otherwise we have to add them here ourselves.
  if (CMAKE_VERSION VERSION_LESS 2.8.11)
    include_directories("${gtest_SOURCE_DIR}/include")
  endif()



  project(blobfusetests)
  find_package(Boost REQUIRED)
  set(Boost_USE_STATIC_LIBS ON)
  # make this configurable by passing param to build.sh. Dont set an enviroment variable because that will go in the cmakecache
  set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -O0 -ggdb")
  find_package(Boost COMPONENTS filesystem system REQUIRED)
  find_package(Boost COMPONENTS thread REQUIRED)
  add_definitions(-std=c++11)
  pkg_search_module(UUID REQUIRED uuid)
  include_directories(${Boost_INCLUDE_DIR})
  add_executable(blobfusetests ${BLOBFUSE_HEADER} ${BLOBFUSE_SOURCE} blobfuse/blobfuse.cpp test/cpplitetests.cpp test/attribcachetests.cpp test/attribcachesynchronizationtests.cpp test/oauthtokentests.cpp test/oauthtokencredentialmanagertests.cpp)
  add_dependencies (blobfusetests azure-storage-lite)
  target_link_libraries(blobfusetests ${CURL_LIBRARIES} ${GNUTLS_LIBRARIES} ${CMAKE_THREAD_LIBS_INIT} ${UUID_LIBRARIES} ${Boost_FILESYSTEM_LIBRARY} ${Boost_SYSTEM_LIBRARY} ${Boost_THREAD_LIBRARY} azure-storage-lite fuse gcrypt gmock_main)

endif()
