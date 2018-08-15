cmake_minimum_required(VERSION 2.8)
set(AZURE_STORAGE_HEADER
  include/storage_EXPORTS.h

  include/base64.h
  include/common.h
  include/constants.h
  include/constants.dat
  include/executor.h
  include/hash.h
  include/retry.h
  include/utility.h

  include/tinyxml2.h
  include/xml_parser_base.h
  include/tinyxml2_parser.h
  include/xml_writer.h

  include/storage_account.h
  include/storage_credential.h
  include/storage_outcome.h
  include/storage_stream.h
  include/storage_url.h
  include/storage_errno.h

  include/storage_request_base.h
  include/get_blob_request_base.h
  include/put_blob_request_base.h
  include/delete_blob_request_base.h
  include/copy_blob_request_base.h
  include/create_container_request_base.h
  include/delete_container_request_base.h
  include/list_containers_request_base.h
  include/list_blobs_request_base.h
  include/get_block_list_request_base.h
  include/put_block_request_base.h
  include/get_blob_property_request_base.h
  include/get_container_property_request_base.h
  include/put_block_list_request_base.h
  include/get_container_property_request_base.h
  include/append_block_request_base.h
  include/put_page_request_base.h
  include/get_page_ranges_request_base.h

  include/http_base.h
  include/http/libcurl_http_client.h

  include/blob/blob_client.h
  include/blob/download_blob_request.h
  include/blob/create_block_blob_request.h
  include/blob/delete_blob_request.h
  include/blob/copy_blob_request.h
  include/blob/create_container_request.h
  include/blob/delete_container_request.h
  include/blob/list_containers_request.h
  include/blob/list_blobs_request.h
  include/blob/get_blob_property_request.h
  include/blob/get_container_property_request.h
  include/blob/get_block_list_request.h
  include/blob/put_block_request.h
  include/blob/put_block_list_request.h
  include/blob/append_block_request.h
  include/blob/put_page_request.h
  include/blob/get_page_ranges_request.h

  include/todo/get_blob_metadata_request.h
  include/todo/get_blob_properties_request.h
  include/todo/query_entities_request.h
  include/todo/set_blob_metadata_request.h
)

set(AZURE_STORAGE_SOURCE
  src/base64.cpp
  src/constants.cpp
  src/hash.cpp
  src/utility.cpp

  src/tinyxml2.cpp
  src/tinyxml2_parser.cpp

  src/storage_account.cpp
  src/storage_credential.cpp
  src/storage_url.cpp

  src/get_blob_request_base.cpp
  src/put_blob_request_base.cpp
  src/delete_blob_request_base.cpp
  src/copy_blob_request_base.cpp
  src/create_container_request_base.cpp
  src/delete_container_request_base.cpp
  src/list_containers_request_base.cpp
  src/list_blobs_request_base.cpp
  src/get_blob_property_request_base.cpp
  src/get_block_list_request_base.cpp
  src/get_container_property_request_base.cpp
  src/put_block_request_base.cpp
  src/put_block_list_request_base.cpp
  src/append_block_request_base.cpp
  src/put_page_request_base.cpp
  src/get_page_ranges_request_base.cpp

  src/http/libcurl_http_client.cpp

  src/blob/blob_client.cpp
  src/blob/blob_client_wrapper.cpp
)

if(MSVC)
  add_definitions(-DNOMINMAX)
  include_directories(${CMAKE_SOURCE_DIR}/include ${CMAKE_CURRENT_SOURCE_DIR}/../local/include)
  link_directories(${CMAKE_CURRENT_SOURCE_DIR}/../local/lib)
else(MSVC)

  option(USE_OPENSSL "Use OpenSSL instead of GnuTLS" OFF)

  find_package(Threads REQUIRED)
  find_package(CURL REQUIRED)
  find_package(OpenSSL REQUIRED)
  pkg_search_module(UUID REQUIRED uuid)

  if (NOT ${USE_OPENSSL})
    find_package(GnuTLS REQUIRED)
    set(EXTRA_INCLUDE ${GNUTLS_INCLUDE_DIR})
    set(EXTRA_LIBRARIES ${GNUTLS_LIBRARIES})
  else()
    add_definitions(-DUSE_OPENSSL)
  endif()

  add_definitions(-std=c++11)
  set(WARNING "-Wall -Wextra -Werror -pedantic -pedantic-errors")
  set(CMAKE_CXX_FLAGS "${CMAKE_THREAD_LIBS_INIT} ${WARNING} ${CMAKE_CXX_FLAGS}")
  include_directories(${CMAKE_SOURCE_DIR}/include ${CURL_INCLUDE_DIRS} ${OPENSSL_INCLUDE_DIR} ${EXTRA_INCLUDE})
endif(MSVC)

set(CMAKE_MACOSX_RPATH ON)
add_library(azure-storage SHARED ${AZURE_STORAGE_HEADER} ${AZURE_STORAGE_SOURCE})

if(MSVC)
  target_link_libraries(azure-storage bcrypt libcurl)
else(MSVC)
  target_link_libraries(azure-storage ${CURL_LIBRARIES} ${UUID_LIBRARIES} ${OPENSSL_CRYPTO_LIBRARY} ${OPENSSL_SSL_LIBRARY} ${EXTRA_LIBRARIES})
endif(MSVC)

add_subdirectory(sample)