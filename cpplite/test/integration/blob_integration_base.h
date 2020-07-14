#pragma once

#include "../test_base.h"

namespace as_test {
    std::string create_random_container(const std::string& prefix, azure::storage_lite::blob_client& client);
    
    std::vector<std::string> create_random_containers(const std::string& prefix, azure::storage_lite::blob_client& client, size_t count);

    std::string get_base64_block_id(unsigned id);
}
