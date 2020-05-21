cmake_minimum_required(VERSION 2.8)
option(BUILD_TESTS       "Build test codes"                  OFF)
option(BUILD_SAMPLES     "Build sample codes"                OFF)
option(BUILD_SHARED_LIBS "Request build of shared libraries" OFF)
set(AZURE_STORAGE_HEADER
  include/storage_EXPORTS.h

  include/logging.h
  include/base64.h
  include/common.h
  include/compare.h
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
  src/logging.cpp
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

if(APPLE)
  option(USE_OPENSSL "Use OpenSSL instead of GnuTLS" ON)
  add_definitions(-DUSE_OPENSSL)
  find_package(CURL REQUIRED)
  # Adding default brew install DIR for openssl
  set(OPENSSL_ROOT_DIR ${OPENSSL_ROOT_DIR} "/usr/local/opt/openssl/")
  find_package(OpenSSL REQUIRED)
  include_directories(${PROJECT_SOURCE_DIR}/include ${CURL_INCLUDE_DIRS} ${OPENSSL_INCLUDE_DIR} ${EXTRA_INCLUDE})
  set(EXTRA_LIBRARIES ${EXTRA_LIBRARIES} system)
  set(CMAKE_MACOSX_RPATH ON)
elseif (UNIX)
  option(USE_OPENSSL "Use OpenSSL instead of GnuTLS" OFF)
  find_package(Threads REQUIRED)
  find_package(CURL REQUIRED)
  include(FindPkgConfig)
  pkg_search_module(UUID REQUIRED uuid)
  if (NOT ${USE_OPENSSL})
    find_package(GnuTLS REQUIRED)
    set(EXTRA_INCLUDE ${GNUTLS_INCLUDE_DIR})
    set(EXTRA_LIBRARIES ${GNUTLS_LIBRARIES})
    include_directories(${PROJECT_SOURCE_DIR}/include ${CURL_INCLUDE_DIRS} ${UUID_INCLUDE_DIR} ${EXTRA_INCLUDE})
  else()
    add_definitions(-DUSE_OPENSSL)
    find_package(OpenSSL REQUIRED)
    include_directories(${PROJECT_SOURCE_DIR}/include ${CURL_INCLUDE_DIRS} ${OPENSSL_INCLUDE_DIR} ${UUID_INCLUDE_DIR} ${EXTRA_INCLUDE})
  endif()
else()
  # Add the parameter/definition specifically for Windows.
  option(USE_OPENSSL "Use OpenSSL instead of GnuTLS" ON)
  option(CMAKE_BUILD_TYPE "The default build type" "Debug")
  option(CMAKE_GENERATOR_PLATFORM "The Generator platform used for CMake" "x86")
  #Construct the libcurl folder path
  #This is because the cmake's FindCurl.cmake not being compatible with libcurl's buildscript on Windows.
  #e.g. libcurl-vc15-x64-debug-static-ssl-static-ipv6-sspi
  if((DEFINED CURL_ROOT_DIR) AND (NOT (CURL_ROOT_DIR STREQUAL "")))
    if(CMAKE_CL_64)
      set(INHERIT_ENV "x64")
    else()
      set(INHERIT_ENV "x86")
    endif()

    if(CMAKE_BUILD_TYPE STREQUAL "Debug")
      set(LIBRARY_BUILD_TYPE "debug")
    else()
      set(LIBRARY_BUILD_TYPE "release")
    endif()

    if(CURL_LINK_TYPE STREQUAL "static")
      add_definitions(-DCURL_STATICLIB)
      if(CMAKE_BUILD_TYPE STREQUAL "Debug")
        set(CURL_LIBRARY_NAME "lib\\libcurl_a_debug.lib")
      else()
        set(CURL_LIBRARY_NAME "lib\\libcurl_a.lib")
      endif()
    else()
      if(CMAKE_BUILD_TYPE STREQUAL "Debug")
        set(CURL_LIBRARY_NAME "bin\\libcurl_debug.dll")
      else()
        set(CURL_LIBRARY_NAME "bin\\libcurl.dll")
      endif()
    endif()

    set(CURL_LIBRARIES "${CURL_ROOT_DIR}\\builds\\libcurl-vc15-${INHERIT_ENV}-${LIBRARY_BUILD_TYPE}-${CURL_LINK_TYPE}-ssl-${OPENSSL_LINK_TYPE}-ipv6-sspi\\${CURL_LIBRARY_NAME}")
    set(CURL_INCLUDE_DIRS "${CURL_ROOT_DIR}\\builds\\libcurl-vc15-${INHERIT_ENV}-${LIBRARY_BUILD_TYPE}-${CURL_LINK_TYPE}-ssl-${OPENSSL_LINK_TYPE}-ipv6-sspi\\include\\")

    message (STATUS "Constructed curl library path: ${CURL_LIBRARIES}")
    message (STATUS "Constructed curl include path: ${CURL_INCLUDE_DIRS}")

    unset(INHERIT_ENV)
    unset(LIBRARY_BUILD_TYPE)
    unset(CURL_LIBRARY_NAME)
  else()
    if(CURL_LINK_TYPE STREQUAL "static")
      add_definitions(-DCURL_STATICLIB)
    endif()

        #if there isn't a customized openssl library and include, then use FindOpenSSL.cmake to find Openssl on Windows
    if(NOT((DEFINED CURL_INCLUDE_DIRS) OR (DEFINED CURL_LIBRARIES)))
      find_package(CURL REQUIRED)
    endif()
  endif()

  #Constructing OPENSSL path on windows.
  if((DEFINED OPENSSL_ROOT_DIR) AND (NOT (OPENSSL_ROOT_DIR STREQUAL "")))
    if(CMAKE_CL_64)
      set(INHERIT_ENV "x64")
    else()
      set(INHERIT_ENV "x86")
    endif()

    if(OPENSSL_LINK_TYPE STREQUAL "static")
      add_definitions(-DOPENSSL_STATICLIB)
      set(OPENSSL_LIBRARY_NAME "ssleay32.lib")
      set(OPENSSL_CRYPTO_LIBRARY_NAME "libeay32.lib")
    else()
      set(OPENSSL_LIBRARY_NAME "ssleay32.dll")
      set(OPENSSL_CRYPTO_LIBRARY_NAME "libeay32.dll")
    endif()

    set(OPENSSL_SSL_LIBRARY "${OPENSSL_ROOT_DIR}\\lib\\${CMAKE_BUILD_TYPE}\\${OPENSSL_LIBRARY_NAME}")
    set(OPENSSL_CRYPTO_LIBRARY "${OPENSSL_ROOT_DIR}\\lib\\${CMAKE_BUILD_TYPE}\\${OPENSSL_CRYPTO_LIBRARY_NAME}")
    set(OPENSSL_INCLUDE_DIR "${OPENSSL_ROOT_DIR}\\include\\openssl\\")
    message (STATUS "Constructed openssl library path: ${OPENSSL_SSL_LIBRARY}")
    message (STATUS "Constructed openssl include path: ${OPENSSL_INCLUDE_DIR}")
    message (STATUS "Constructed openssl crypto library path: ${OPENSSL_CRYPTO_LIBRARY}")
    unset(INHERIT_ENV)
    unset(OPENSSL_LIBRARY_NAME)
    unset(OPENSSL_CRYPTO_LIBRARY_NAME)
  else()
    if(OPENSSL_LINK_TYPE STREQUAL "static")
      add_definitions(-DOPENSSL_STATICLIB)
    endif()

    #if there isn't a customized openssl library and include, then use FindOpenSSL.cmake to find Openssl on Windows
    if(NOT((DEFINED OPENSSL_INCLUDE_DIR) AND (DEFINED OPENSSL_CRYPTO_LIBRARY) AND (DEFINED OPENSSL_SSL_LIBRARY)))
      message(STATUS "OpenSSL folder is not specified, need to find it automatically.")
      find_package(OpenSSL REQUIRED)
    endif()
  endif()

  if(OPENSSL_LINK_TYPE STREQUAL "static")
    add_definitions(-DOPENSSL_STATICLIB)
  endif()
  add_definitions(-DUSE_OPENSSL)
  set(OPENSSL_MSVC_STATIC_RT "true")
  if (BUILD_WITH_MT)
    set(CMAKE_CXX_FLAGS_RELEASE "/MT")
    set(CMAKE_CXX_FLAGS_DEBUG "/MT")
  endif()
  #adding extra dependency libraries for windows
  set(EXTRA_LIBRARIES ${EXTRA_LIBRARIES} rpcrt4 ws2_32 wldap32 crypt32 normaliz)
  include_directories(${PROJECT_SOURCE_DIR}/include ${CURL_INCLUDE_DIRS} ${OPENSSL_INCLUDE_DIR} ${EXTRA_INCLUDE})
endif()

if (("${CMAKE_CXX_COMPILER_ID}" STREQUAL "Clang") OR ("${CMAKE_CXX_COMPILER_ID}" STREQUAL "GNU"))
  add_definitions(-std=c++11)
  set(WARNING "-Wall -Wextra -Werror -pedantic -pedantic-errors")
  set(CMAKE_CXX_FLAGS "${CMAKE_THREAD_LIBS_INIT} ${WARNING} ${CMAKE_CXX_FLAGS}")
endif()

add_library(azure-storage-lite ${AZURE_STORAGE_LITE_HEADER} ${AZURE_STORAGE_LITE_SOURCE})

if (NOT ${USE_OPENSSL})
  target_link_libraries(azure-storage-lite ${CURL_LIBRARIES} ${UUID_LIBRARIES} ${EXTRA_LIBRARIES})
else()
  target_link_libraries(azure-storage-lite ${CURL_LIBRARIES} ${OPENSSL_CRYPTO_LIBRARY} ${OPENSSL_SSL_LIBRARY} ${UUID_LIBRARIES} ${EXTRA_LIBRARIES})
endif()

if(BUILD_TESTS)
  add_subdirectory(test)
endif()

if(BUILD_SAMPLES)
  add_subdirectory(sample)
endif()

if (WIN32 OR UNIX)
  file(GLOB BLOB_HEADERS includes/blob/*.h)
  install(FILES ${BLOB_HEADERS} DESTINATION include/blob)

  file(GLOB HTTP_HEADERS includes/http/*.h)
  install(FILES ${HTTP_HEADERS} DESTINATION include/http)

  file(GLOB TODO_HEADERS includes/todo/*.h)
  install(FILES ${HTTP_HEADERS} DESTINATION todo/http)

  file(GLOB GENERAL_HEADERS includes/*.h)
  install(FILES ${GENERAL_HEADERS} DESTINATION include)

  file(GLOB GENERAL_DATA includes/*.dat)
  install(FILES ${GENERAL_DATA} DESTINATION include)
endif()

# Reconfigure final output directory
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${PROJECT_BINARY_DIR}/Binaries)
set(CMAKE_LIBRARY_OUTPUT_DIRECTORY ${PROJECT_BINARY_DIR}/Binaries)
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${PROJECT_BINARY_DIR}/Binaries)

# Set version numbers centralized
set (AZURE_STORAGE_LITE_VERSION_MAJOR 0)
set (AZURE_STORAGE_LITE_VERSION_MINOR 1)
set (AZURE_STORAGE_LITE_VERSION_REVISION 0)

# Set output directories.
if(NOT DEFINED CMAKE_INSTALL_BINDIR)
  set(CMAKE_INSTALL_BINDIR bin)
endif()

if(NOT DEFINED CMAKE_INSTALL_LIBDIR)
  set(CMAKE_INSTALL_LIBDIR lib)
endif()
