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
)

set (BLOBFUSE_HEADER
  blobfuse/blobfuse.h
)

set (BLOBFUSE_SOURCE
  blobfuse/blobfuse.cpp
  blobfuse/fileapis.cpp
  blobfuse/directoryapis.cpp
  blobfuse/utilities.cpp
)

if(UNIX)
  project(blobfuse)
  find_package(Threads REQUIRED)
  find_package(CURL REQUIRED)
  find_package(GnuTLS REQUIRED)
  add_definitions(-std=c++11)
  add_definitions(-D_FILE_OFFSET_BITS=64)
  set(WARNING "-Wall -Wextra -Werror -pedantic -pedantic-errors")
  set(CMAKE_CXX_FLAGS "${CMAKE_THREAD_LIBS_INIT} ${WARNING} ${CMAKE_CXX_FLAGS}")
  include_directories(${CMAKE_SOURCE_DIR}/blobfuse ${CMAKE_SOURCE_DIR}/azure-storage-cpp-lite/include ${CURL_INCLUDE_DIRS} ${GNUTLS_INCLUDE_DIR})

  set(CMAKE_MACOSX_RPATH ON)
  add_executable(blobfuse ${BLOBFUSE_HEADER} ${BLOBFUSE_SOURCE} ${AZURE_STORAGE_HEADER} ${AZURE_STORAGE_SOURCE})

  target_link_libraries(blobfuse ${CURL_LIBRARIES} ${GNUTLS_LIBRARIES} ${CMAKE_THREAD_LIBS_INIT} fuse)
  install(TARGETS blobfuse
    PERMISSIONS OWNER_EXECUTE OWNER_WRITE OWNER_READ GROUP_READ GROUP_EXECUTE WORLD_READ WORLD_EXECUTE
    DESTINATION bin)

  set(CPACK_GENERATOR "DEB")
  set(CPACK_DEBIAN_PACKAGE_MAINTAINER "Microsoft - Azure Storage")
  include(CPack)
endif(UNIX)
