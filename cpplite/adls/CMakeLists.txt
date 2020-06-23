cmake_minimum_required(VERSION 2.8)

project(azure_storage_adls)

set(AZURE_STORAGE_ADLS_SOURCES
  include/json.hpp
  include/nlohmann_json_parser.h
  include/adls_client.h
  include/adls_request_base.h
  include/create_directory_request.h
  include/delete_directory_request.h
  include/set_access_control_request.h
  include/get_access_control_request.h
  include/list_paths_request.h
  include/create_file_request.h
  include/append_data_request.h
  include/flush_data_request.h
  include/rename_file_request.h

  src/nlohmann_json_parser.cpp
  src/adls_client.cpp
  src/create_directory_request.cpp
  src/delete_directory_request.cpp
  src/set_access_control_request.cpp
  src/get_access_control_request.cpp
  src/list_paths_request.cpp
  src/create_file_request.cpp
  src/append_data_request.cpp
  src/flush_data_request.cpp
  src/rename_file_request.cpp
)

add_library(azure-storage-adls ${AZURE_STORAGE_ADLS_SOURCES})

target_include_directories(azure-storage-adls PUBLIC include)
if(WIN32)
  target_compile_definitions(azure-storage-adls PRIVATE azure_storage_adls_EXPORTS NOMINMAX)
endif()
target_link_libraries(azure-storage-adls azure-storage-lite)


if(BUILD_TESTS)
  set(AZURE_STORAGE_ADLS_TEST_SOURCES
    test/adls_test_base.h
    test/client_test.cpp
    test/filesystem_test.cpp
    test/directory_test.cpp
    test/file_test.cpp
  )
  string(REGEX REPLACE "([^;]+)" "${CMAKE_CURRENT_SOURCE_DIR}/\\1" AZURE_STORAGE_ADLS_TEST_SOURCES "${AZURE_STORAGE_ADLS_TEST_SOURCES}")
  set(AZURE_STORAGE_ADLS_TEST_SOURCES ${AZURE_STORAGE_ADLS_TEST_SOURCES} PARENT_SCOPE)
endif()
