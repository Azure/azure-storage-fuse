#include "blob_integration_base.h"

#include "catch2/catch.hpp"

// List all blobs that returns a iterator is going to be supported in the future, and this test case set will be valid again.

//TEST_CASE("List blobs", "[blob],[blob_service]")
//{
//    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
//    std::string container_name = as_test::create_random_container("", client);
//    unsigned blob_count = 15;
//    std::vector<std::string> blobs;
//    std::string blob_prefix_1 = as_test::get_random_string(20);
//    std::string blob_prefix_2 = as_test::get_random_string(20);
//    for (unsigned i = 0; i < blob_count; ++i)
//    {
//        auto blob_name_1 = blob_prefix_1 + as_test::get_random_string(10);
//        auto blob_name_2 = blob_prefix_2 + as_test::get_random_string(10);
//        {
//            auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name_1, 512).get();
//            REQUIRE(create_page_blob_outcome.success());
//            blobs.push_back(blob_name_1);
//        }
//        {
//            auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name_2, 1024).get();
//            REQUIRE(create_page_blob_outcome.success());
//            blobs.push_back(blob_name_2);
//        }
//    }
//
//    SECTION("List Blobs successfully") {
//        auto list_blob_outcome = client.list_blobs(container_name, "").get();
//        REQUIRE(list_blob_outcome.success());
//        REQUIRE(!list_blob_outcome.response().next_marker.empty());
//        auto listed_blobs = list_blob_outcome.response().blobs;
//        REQUIRE(listed_blobs.size() == 2);// list_blobs does not do what it said to do.
//        for (auto blob : listed_blobs)
//        {
//            REQUIRE(((blob.content_length == 1024) || (blob.content_length == 512)));
//            REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
//        }
//    }
//
//    SECTION("List Blobs with prefix successfully") {
//        {
//            auto list_blob_outcome = client.list_blobs(container_name, blob_prefix_1).get();
//            REQUIRE(list_blob_outcome.success());
//            REQUIRE(!list_blob_outcome.response().next_marker.empty());
//            auto listed_blobs = list_blob_outcome.response().blobs;
//            REQUIRE(listed_blobs.size() == 2);
//            for (auto blob : listed_blobs)
//            {
//                REQUIRE(blob.content_length == 512);
//                REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
//            }
//        }
//
//        {
//            auto list_blob_outcome = client.list_blobs(container_name, blob_prefix_2).get();
//            REQUIRE(list_blob_outcome.success());
//            REQUIRE(!list_blob_outcome.response().next_marker.empty());
//            auto listed_blobs = list_blob_outcome.response().blobs;
//            REQUIRE(listed_blobs.size() == 2);
//            for (auto blob : listed_blobs)
//            {
//                REQUIRE(blob.content_length == 1024);
//                REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
//            }
//        }
//    }
//
//    SECTION("List Blobs with invalid prefix successfully") {
//        auto list_blob_outcome = client.list_blobs(container_name, "1~invalid~~%d_prefix").get();
//        REQUIRE(list_blob_outcome.success());
//        REQUIRE(list_blob_outcome.response().next_marker.empty());
//        REQUIRE(list_blob_outcome.response().blobs.empty());
//    }
//
//    client.delete_container(container_name);
//}

TEST_CASE("Storage endpoint", "[endpoint]")
{
    std::string account_name = "account";
    std::string account_key = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=";
    auto cred = std::make_shared<azure::storage_lite::shared_key_credential>(account_name, account_key);

    struct test_case
    {
        bool use_https;
        std::string endpoint;

        std::string domain;
        std::string path;
        std::string url;
    };
    std::vector<test_case> test_cases
    {
        {true, "", "https://account.blob.core.windows.net", "", "https://account.blob.core.windows.net"},
        {false, "", "http://account.blob.core.windows.net", "", "http://account.blob.core.windows.net"},

        {true, "www.example.com", "https://www.example.com", "", "https://www.example.com"},
        {false, "127.0.0.1", "http://127.0.0.1", "", "http://127.0.0.1"},

        {true, "127.0.0.1/account", "https://127.0.0.1", "/account", "https://127.0.0.1/account"},
        {false, "127.0.0.1:8888/account", "http://127.0.0.1:8888", "/account", "http://127.0.0.1:8888/account"},
        {false, "https://127.0.0.1:8888/account", "http://127.0.0.1:8888", "/account", "http://127.0.0.1:8888/account"},
    };

    for (const auto& c: test_cases)
    {
        auto account = std::make_shared<azure::storage_lite::storage_account>(account_name, cred, c.use_https, c.endpoint);
        CHECK(c.domain == account->get_url(azure::storage_lite::storage_account::service::blob).get_domain());
        CHECK(c.path == account->get_url(azure::storage_lite::storage_account::service::blob).get_path());
        CHECK(c.url == account->get_url(azure::storage_lite::storage_account::service::blob).to_string());
    }
}

TEST_CASE("List blobs segmented", "[blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    unsigned blob_count = 15;
    std::vector<std::string> blobs;
    std::string blob_prefix_1 = as_test::get_random_string(20);
    std::string blob_prefix_2 = as_test::get_random_string(20);
    for (unsigned i = 0; i < blob_count; ++i)
    {
        auto blob_name_1 = blob_prefix_1 + "/" + as_test::get_random_string(10);
        auto blob_name_2 = blob_prefix_2 + "-" + as_test::get_random_string(10);
        {
            auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name_1, 512).get();
            REQUIRE(create_page_blob_outcome.success());
            blobs.push_back(blob_name_1);
        }
        {
            auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name_2, 1024).get();
            REQUIRE(create_page_blob_outcome.success());
            blobs.push_back(blob_name_2);
        }
    }

    SECTION("List blobs segmented successfully")
    {
        auto list_blob_outcome = client.list_blobs_segmented(container_name, "", "", "").get();
        REQUIRE(list_blob_outcome.success());
        auto marker = list_blob_outcome.response().next_marker;
        auto listed_blobs = list_blob_outcome.response().blobs;
        while (!marker.empty())
        {
            auto list_again_blob_outcome = client.list_blobs_segmented(container_name, "", marker, "").get();
            REQUIRE(list_again_blob_outcome.success());
            auto more_blobs = list_again_blob_outcome.response().blobs;
            listed_blobs.reserve(listed_blobs.size() + more_blobs.size());
            listed_blobs.insert(listed_blobs.end(), more_blobs.begin(), more_blobs.end());
            marker = list_again_blob_outcome.response().next_marker;
        }
        REQUIRE(marker.empty());
        REQUIRE(listed_blobs.size() == 30);
        for (auto blob : listed_blobs)
        {
            REQUIRE(((blob.content_length == 1024) || (blob.content_length == 512)));
            REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
        }
    }

    SECTION("List blobs segmented with prefix successfully")
    {
        {
            auto list_blob_outcome = client.list_blobs_segmented(container_name, "", "", blob_prefix_1, 2).get();
            REQUIRE(list_blob_outcome.success());
            auto marker = list_blob_outcome.response().next_marker;
            auto listed_blobs = list_blob_outcome.response().blobs;
            while (!marker.empty())
            {
                auto list_again_blob_outcome = client.list_blobs_segmented(container_name, "", marker, blob_prefix_1, 2).get();
                REQUIRE(list_again_blob_outcome.success());
                auto more_blobs = list_again_blob_outcome.response().blobs;
                listed_blobs.reserve(listed_blobs.size() + more_blobs.size());
                listed_blobs.insert(listed_blobs.end(), more_blobs.begin(), more_blobs.end());
                marker = list_again_blob_outcome.response().next_marker;
            }
            REQUIRE(marker.empty());
            REQUIRE(listed_blobs.size() == 15);
            for (auto blob : listed_blobs)
            {
                REQUIRE(blob.content_length == 512);
                REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
            }
        }

        {
            auto list_blob_outcome = client.list_blobs_segmented(container_name, "", "", blob_prefix_2, 2).get();
            REQUIRE(list_blob_outcome.success());
            auto marker = list_blob_outcome.response().next_marker;
            auto listed_blobs = list_blob_outcome.response().blobs;
            while (!marker.empty())
            {
                auto list_again_blob_outcome = client.list_blobs_segmented(container_name, "", marker, blob_prefix_2, 2).get();
                REQUIRE(list_again_blob_outcome.success());
                auto more_blobs = list_again_blob_outcome.response().blobs;
                listed_blobs.reserve(listed_blobs.size() + more_blobs.size());
                listed_blobs.insert(listed_blobs.end(), more_blobs.begin(), more_blobs.end());
                marker = list_again_blob_outcome.response().next_marker;
            }
            REQUIRE(marker.empty());
            REQUIRE(listed_blobs.size() == 15);
            for (auto blob : listed_blobs)
            {
                REQUIRE(blob.content_length == 1024);
                REQUIRE(std::find(blobs.begin(), blobs.end(), blob.name) != blobs.end());
            }
        }
    }
    client.delete_container(container_name);
}

TEST_CASE("Get blob property", "[blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_blob_outcome = client.create_page_blob(container_name, blob_name, 1024).get();
    REQUIRE(create_blob_outcome.success());

    SECTION("Get blob property for existing blob successfully")
    {
        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 1024);

        std::vector<std::pair<std::string, std::string>> metadata;
        metadata.emplace_back(std::make_pair("mkey1", "mvalue1"));
        metadata.emplace_back(std::make_pair("mkEy2", "MValUe2#  % %2d"));
        auto set_metadata_outcome = client.set_blob_metadata(container_name, blob_name, metadata).get();
        REQUIRE(set_metadata_outcome.success());

        get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().metadata == metadata);

        metadata.clear();
        set_metadata_outcome = client.set_blob_metadata(container_name, blob_name, metadata).get();
        REQUIRE(set_metadata_outcome.success());

        get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().metadata == metadata);
    }

    SECTION("Get blob property for non-existing blob successfully")
    {
        auto get_blob_property_outcome = client.get_blob_properties(container_name, as_test::get_random_string(20)).get();
        REQUIRE_FALSE(get_blob_property_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Delete blob", "[blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_blob_outcome = client.create_page_blob(container_name, blob_name, 1024).get();
    REQUIRE(create_blob_outcome.success());

    SECTION("Delete existing blob successfully")
    {
        auto delete_blob_outcome = client.delete_blob(container_name, blob_name).get();
        REQUIRE(delete_blob_outcome.success());
    }

    SECTION("Delete non-existing blob successfully")
    {
        auto delete_blob_outcome = client.delete_blob(container_name, as_test::get_random_string(20)).get();
        REQUIRE(!delete_blob_outcome.success());
        REQUIRE(delete_blob_outcome.error().code == "404");
        REQUIRE(delete_blob_outcome.error().code_name == "BlobNotFound");
    }

    client.delete_container(container_name);
}

TEST_CASE("Start copy blob", "[blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto iss = as_test::get_istringstream_with_random_buffer(2048);
    auto create_blob_outcome = client.upload_block_blob_from_stream(container_name, blob_name, iss, std::vector<std::pair<std::string, std::string>>()).get();
    REQUIRE(create_blob_outcome.success());

    SECTION("Start copy blob successfully")
    {
        std::string dest_blob_name = as_test::get_random_string(20);
        auto start_copy_outcome = client.start_copy(container_name, blob_name, container_name, dest_blob_name).get();
        REQUIRE(start_copy_outcome.success());
        auto get_blob_property_outcome = client.get_blob_properties(container_name, dest_blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 2048);
    }

    client.delete_container(container_name);
}
