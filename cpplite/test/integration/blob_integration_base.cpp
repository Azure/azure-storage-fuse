#include "blob_integration_base.h"

#include "../test_constants.h"

namespace as_test {
    std::string create_random_container(const std::string& prefix, azure::storage_lite::blob_client& client)
    {
        //Assume prefix is less than max prefix size.
        auto container_name = prefix + get_random_string(MAX_PREFIX_SIZE - prefix.size());
        auto result = client.create_container(container_name).get();
        if (!result.success())
        {
            return create_random_container(prefix, client);
        }
        return container_name;
    }

    std::vector<std::string> create_random_containers(const std::string& prefix, azure::storage_lite::blob_client& client, size_t count)
    {
        std::vector<std::string> results;
        for (size_t i = 0; i < count; ++i)
        {
            results.push_back(create_random_container(prefix, client));
        }
        return results;
    }

    std::string get_base64_block_id(unsigned id)
    {
        std::string raw_block_id = std::to_string(id);
        //pad the string to length of 6.
        raw_block_id.insert(raw_block_id.begin(), 6 - raw_block_id.length(), '0');
        return to_base64(raw_block_id.c_str(), 6);
    }
}
