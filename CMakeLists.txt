cmake_minimum_required(VERSION 2.8)
set(AZURE_STORAGE_HEADER
  azure-storage-cpp-lite/include/storage_EXPORTS.h

  azure-storage-cpp-lite/include/base64.h
  azure-storage-cpp-lite/include/common.h
  azure-storage-cpp-lite/include/constants.h
  azure-storage-cpp-lite/include/constants.dat
  azure-storage-cpp-lite/include/executor.h
  azure-storage-cpp-lite/include/hash.h
  azure-storage-cpp-lite/include/retry.h
  azure-storage-cpp-lite/include/utility.h

  azure-storage-cpp-lite/include/tinyxml2.h
  azure-storage-cpp-lite/include/xml_parser_base.h
  azure-storage-cpp-lite/include/tinyxml2_parser.h
  azure-storage-cpp-lite/include/xml_writer.h

  azure-storage-cpp-lite/include/storage_account.h
  azure-storage-cpp-lite/include/storage_credential.h
  azure-storage-cpp-lite/include/storage_outcome.h
  azure-storage-cpp-lite/include/storage_stream.h
  azure-storage-cpp-lite/include/storage_url.h
  azure-storage-cpp-lite/include/storage_errno.h

  azure-storage-cpp-lite/include/storage_request_base.h
  azure-storage-cpp-lite/include/get_blob_request_base.h
  azure-storage-cpp-lite/include/put_blob_request_base.h
  azure-storage-cpp-lite/include/delete_blob_request_base.h
  azure-storage-cpp-lite/include/copy_blob_request_base.h
  azure-storage-cpp-lite/include/create_container_request_base.h
  azure-storage-cpp-lite/include/delete_container_request_base.h
  azure-storage-cpp-lite/include/list_containers_request_base.h
  azure-storage-cpp-lite/include/list_blobs_request_base.h
  azure-storage-cpp-lite/include/get_block_list_request_base.h
  azure-storage-cpp-lite/include/put_block_request_base.h
  azure-storage-cpp-lite/include/get_blob_property_request_base.h
  azure-storage-cpp-lite/include/get_container_property_request_base.h
  azure-storage-cpp-lite/include/put_block_list_request_base.h
  azure-storage-cpp-lite/include/get_container_property_request_base.h
  azure-storage-cpp-lite/include/append_block_request_base.h
  azure-storage-cpp-lite/include/put_page_request_base.h
  azure-storage-cpp-lite/include/get_page_ranges_request_base.h

  azure-storage-cpp-lite/include/http_base.h
  azure-storage-cpp-lite/include/http/libcurl_http_client.h

  azure-storage-cpp-lite/include/blob/blob_client.h
  azure-storage-cpp-lite/include/blob/download_blob_request.h
  azure-storage-cpp-lite/include/blob/create_block_blob_request.h
  azure-storage-cpp-lite/include/blob/delete_blob_request.h
  azure-storage-cpp-lite/include/blob/copy_blob_request.h
  azure-storage-cpp-lite/include/blob/create_container_request.h
  azure-storage-cpp-lite/include/blob/delete_container_request.h
  azure-storage-cpp-lite/include/blob/list_containers_request.h
  azure-storage-cpp-lite/include/blob/list_blobs_request.h
  azure-storage-cpp-lite/include/blob/get_blob_property_request.h
  azure-storage-cpp-lite/include/blob/get_container_property_request.h
  azure-storage-cpp-lite/include/blob/get_block_list_request.h
  azure-storage-cpp-lite/include/blob/put_block_request.h
  azure-storage-cpp-lite/include/blob/put_block_list_request.h
  azure-storage-cpp-lite/include/blob/append_block_request.h
  azure-storage-cpp-lite/include/blob/put_page_request.h
  azure-storage-cpp-lite/include/blob/get_page_ranges_request.h

  azure-storage-cpp-lite/include/todo/get_blob_metadata_request.h
  azure-storage-cpp-lite/include/todo/get_blob_properties_request.h
  azure-storage-cpp-lite/include/todo/query_entities_request.h
  azure-storage-cpp-lite/include/todo/set_blob_metadata_request.h
)

set(AZURE_STORAGE_SOURCE
  azure-storage-cpp-lite/src/base64.cpp
  azure-storage-cpp-lite/src/constants.cpp
  azure-storage-cpp-lite/src/hash.cpp
  azure-storage-cpp-lite/src/utility.cpp

  azure-storage-cpp-lite/src/tinyxml2.cpp
  azure-storage-cpp-lite/src/tinyxml2_parser.cpp

  azure-storage-cpp-lite/src/storage_account.cpp
  azure-storage-cpp-lite/src/storage_credential.cpp
  azure-storage-cpp-lite/src/storage_url.cpp

  azure-storage-cpp-lite/src/get_blob_request_base.cpp
  azure-storage-cpp-lite/src/put_blob_request_base.cpp
  azure-storage-cpp-lite/src/delete_blob_request_base.cpp
  azure-storage-cpp-lite/src/copy_blob_request_base.cpp
  azure-storage-cpp-lite/src/create_container_request_base.cpp
  azure-storage-cpp-lite/src/delete_container_request_base.cpp
  azure-storage-cpp-lite/src/list_containers_request_base.cpp
  azure-storage-cpp-lite/src/list_blobs_request_base.cpp
  azure-storage-cpp-lite/src/get_blob_property_request_base.cpp
  azure-storage-cpp-lite/src/get_block_list_request_base.cpp
  azure-storage-cpp-lite/src/get_container_property_request_base.cpp
  azure-storage-cpp-lite/src/put_block_request_base.cpp
  azure-storage-cpp-lite/src/put_block_list_request_base.cpp
  azure-storage-cpp-lite/src/append_block_request_base.cpp
  azure-storage-cpp-lite/src/put_page_request_base.cpp
  azure-storage-cpp-lite/src/get_page_ranges_request_base.cpp

  azure-storage-cpp-lite/src/http/libcurl_http_client.cpp

  azure-storage-cpp-lite/src/blob/blob_client.cpp
  azure-storage-cpp-lite/src/blob/blob_client_wrapper.cpp
  azure-storage-cpp-lite/src/blob/blob_client_attr_cache_wrapper.cpp
)

set (BLOBFUSE_HEADER
  blobfuse/blobfuse.h
)

set (BLOBFUSE_SOURCE
#  blobfuse/blobfuse.cpp    # We don't include blobfuse.cpp because the file contents are directly #included in main.cpp.  This allows us to split main() and everything
#                             else into different files, without having to define each function in a header for only one consumer.  The only reason to do all this is so
#                             we can test functionality in blobfuse.cpp without including main() (which would break gtest).
  blobfuse/fileapis.cpp
  blobfuse/directoryapis.cpp
  blobfuse/utilities.cpp
)

if(UNIX)
  project(blobfuse)
  find_package(Threads REQUIRED)
  find_package(CURL REQUIRED)
  find_package(GnuTLS REQUIRED)
  set(CMAKE_CXX_STANDARD 14)
  add_definitions(-D_FILE_OFFSET_BITS=64)
  set(WARNING "-Wall -Wextra -Werror -pedantic -pedantic-errors")
  set(CMAKE_CXX_FLAGS "${CMAKE_THREAD_LIBS_INIT} ${WARNING} ${CMAKE_CXX_FLAGS}")
  include_directories(${CMAKE_SOURCE_DIR}/blobfuse ${CMAKE_SOURCE_DIR}/azure-storage-cpp-lite/include ${CURL_INCLUDE_DIRS} ${GNUTLS_INCLUDE_DIR})

  set(CMAKE_MACOSX_RPATH ON)

  add_executable(blobfuse ${BLOBFUSE_HEADER} ${BLOBFUSE_SOURCE} ${AZURE_STORAGE_HEADER} ${AZURE_STORAGE_SOURCE} blobfuse/main.cpp)

  pkg_search_module(UUID REQUIRED uuid)
  target_link_libraries(blobfuse ${CURL_LIBRARIES} ${GNUTLS_LIBRARIES} ${CMAKE_THREAD_LIBS_INIT} ${UUID_LIBRARIES} fuse gcrypt)
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
  set(CPACK_DEBIAN_PACKAGE_DESCRIPTION "blobfuse 1.1.0 - FUSE adapter for Azure Blob Storage")
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
  set(CMAKE_CXX_STANDARD 14)
  pkg_search_module(UUID REQUIRED uuid)
  add_executable(blobfusetests ${BLOBFUSE_HEADER} ${BLOBFUSE_SOURCE} ${AZURE_STORAGE_HEADER} ${AZURE_STORAGE_SOURCE} blobfuse/blobfuse.cpp test/cpplitetests.cpp test/attribcachetests.cpp test/attribcachesynchronizationtests.cpp)
  target_link_libraries(blobfusetests ${CURL_LIBRARIES} ${GNUTLS_LIBRARIES} ${CMAKE_THREAD_LIBS_INIT} ${UUID_LIBRARIES} fuse gcrypt gmock_main)

endif()
