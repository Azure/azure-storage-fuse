cmake_minimum_required(VERSION 2.8)

add_executable(azure-storage-sample sample.cpp)
if(WIN32)
  target_compile_definitions(azure-storage-sample PRIVATE NOMINMAX)
endif()
target_link_libraries(azure-storage-sample azure-storage-lite)
