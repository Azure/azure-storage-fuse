cmake_minimum_required(VERSION 3.5)

add_executable(azure-storage-sample sample.cpp)
target_link_libraries(azure-storage-sample azure-storage)