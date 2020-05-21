#include "blob_integration_base.h"

#include "catch2/catch.hpp"

TEST_CASE("Create append blob", "[append blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Create append blob with valid name successfully")
    {
        auto create_append_blob_outcome = client.create_append_blob(container_name, blob_name).get();
        REQUIRE(create_append_blob_outcome.success());
    }

    SECTION("Create append blob with invalid container unsuccessfully")
    {
        auto create_append_blob_outcome = client.create_append_blob(as_test::get_random_string(20), blob_name).get();
        REQUIRE(!create_append_blob_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Append block from stream", "[append blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_append_blob_outcome = client.create_append_blob(container_name, blob_name).get();
    REQUIRE(create_append_blob_outcome.success());

    SECTION("Append 4MB block successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto append_block_from_stream_outcome = client.append_block_from_stream(container_name, blob_name, iss).get();
        REQUIRE(append_block_from_stream_outcome.success());
        
        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str());
    }

    SECTION("Append 4MB block 10 times successfully")
    {
        std::string blob_content;
        for (unsigned i = 0; i < 10; ++i)
        {
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            auto append_block_from_stream_outcome = client.append_block_from_stream(container_name, blob_name, iss).get();
            REQUIRE(append_block_from_stream_outcome.success());
            blob_content.append(iss.str());
        }

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 10);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4 * 1024 * 1024 * 10, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == blob_content);
    }

    SECTION("Append 5MB block unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(5 * 1024 * 1024);
        auto append_block_from_stream_outcome = client.append_block_from_stream(container_name, blob_name, iss).get();
        REQUIRE(!append_block_from_stream_outcome.success());
        REQUIRE(append_block_from_stream_outcome.error().code == "413");
        REQUIRE(append_block_from_stream_outcome.error().code_name == "RequestBodyTooLarge");
    }

    SECTION("Append to non-existing blob unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto append_block_from_stream_outcome = client.append_block_from_stream(container_name, as_test::get_random_string(20), iss).get();
        REQUIRE(!append_block_from_stream_outcome.success());
        REQUIRE(append_block_from_stream_outcome.error().code == "404");
        REQUIRE(append_block_from_stream_outcome.error().code_name == "BlobNotFound");
    }

    client.delete_container(container_name);
}
