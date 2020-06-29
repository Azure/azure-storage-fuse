#include "catch2/catch.hpp"

#include "adls_client.h"
#include "adls_test_base.h"

TEST_CASE("Create Filesystem", "[adls][filesystem]")
{
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);
        std::string fs_name = as_test::get_random_string(10);

        client.create_filesystem(fs_name);
        REQUIRE(errno == 0);
        REQUIRE(client.filesystem_exists(fs_name));

        client.delete_filesystem(fs_name);
        REQUIRE(errno == 0);
        REQUIRE_FALSE(client.filesystem_exists(fs_name));

        client.delete_filesystem(fs_name);
        REQUIRE(errno != 0);
    }
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(true);
        std::string fs_name = as_test::get_random_string(10);

        client.create_filesystem(fs_name);
        REQUIRE(client.filesystem_exists(fs_name));
        REQUIRE_THROWS_AS(client.create_filesystem(fs_name), std::exception);
        client.delete_filesystem(fs_name);
        REQUIRE_FALSE(client.filesystem_exists(fs_name));
        REQUIRE(errno == 0);
    }
}

TEST_CASE("List Filesystem", "[adls][filesystem]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string prefix1 = as_test::get_random_string(5);
    std::string prefix2 = as_test::get_random_string(5);
    REQUIRE(prefix1 != prefix2);

    std::set<std::string> fss1;
    std::set<std::string> fss2;
    for (int i = 0; i < 10; ++i)
    {
        std::string fs_name = prefix1 + as_test::get_random_string(10);
        client.create_filesystem(fs_name);
        fss1.insert(fs_name);
        fs_name = prefix2 + as_test::get_random_string(10);
        client.create_filesystem(fs_name);
        fss2.insert(fs_name);
    }

    std::string continuation;
    do
    {
        const size_t segment_size = 5;
        auto list_result = client.list_filesystems_segmented(prefix1, continuation, segment_size);
        REQUIRE(list_result.filesystems.size() <= segment_size);
        continuation = list_result.continuation_token;
        for (const auto& fs : list_result.filesystems)
        {
            auto ite = fss1.find(fs.name);
            REQUIRE(ite != fss1.end());
            fss1.erase(ite);
        }
    } while (!continuation.empty());

    REQUIRE(fss1.empty());

    for (const auto& fs_name : fss1)
    {
        client.delete_filesystem(fs_name);
    }
    for (const auto& fs_name : fss2)
    {
        client.delete_filesystem(fs_name);
    }
}

TEST_CASE("Filesystem Properties", "[adls][filesystem]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);

    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("mkey1", "mvalue1"));
    metadata.emplace_back(std::make_pair("mKey2", "mvAlue1    $%^&#"));
    client.set_filesystem_properties(fs_name, metadata);
    REQUIRE(errno == 0);

    auto metadata2 = client.get_filesystem_properties(fs_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    metadata.clear();
    client.set_filesystem_properties(fs_name, metadata);
    REQUIRE(errno == 0);

    metadata2 = client.get_filesystem_properties(fs_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    client.delete_filesystem(fs_name);
}