cmake_minimum_required(VERSION 2.8)

project(azure_storage_test)

set(AZURE_STORAGE_TEST_SOURCES
  test_constants.h
  test_base.h
  integration/blob_integration_base.h

  test_constants.cpp
  test_base.cpp
  integration/blob_integration_base.cpp
  integration/blob_container_integration_test.cpp
  integration/blob_general_integration_test.cpp
  integration/block_blob_integration_test.cpp
  integration/append_blob_integration_test.cpp
  integration/page_blob_integration_test.cpp
)

add_executable(azure-storage-test ${AZURE_STORAGE_TEST_SOURCES})
if(WIN32)
  target_compile_definitions(azure-storage-test PRIVATE NOMINMAX)
endif()
target_link_libraries(azure-storage-test azure-storage-lite Catch2::Catch2)
if (TARGET azure-storage-adls)
  target_sources(azure-storage-test PRIVATE ${AZURE_STORAGE_ADLS_TEST_SOURCES})
  target_link_libraries(azure-storage-test azure-storage-adls)
endif()
