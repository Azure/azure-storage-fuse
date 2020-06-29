#include "blob_integration_base.h"

#include "catch2/catch.hpp"

TEST_CASE("Create page blob", "[page blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Create page blob with valid name successfully")
    {
        auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, 1024).get();
        REQUIRE(create_page_blob_outcome.success());
    }

    SECTION("Create page blob with invalid container unsuccessfully")
    {
        auto create_page_blob_outcome = client.create_page_blob(as_test::get_random_string(20), blob_name, 1024).get();
        REQUIRE(!create_page_blob_outcome.success());
    }

    SECTION("Create page blob with too large a size unsuccessfully")
    {
        long long size = 1024ll * 1024ll * 1024ll * 8ll + 1ll;
        auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, size).get();
        REQUIRE(!create_page_blob_outcome.success());
    }

    SECTION("Create 8TB page blob successfully")
    {
        long long size = 1024ll * 1024ll * 1024ll * 8ll;
        auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, size).get();
        REQUIRE(create_page_blob_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Put page from stream", "[page blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, 64 * 1024 * 1024).get();
    REQUIRE(create_page_blob_outcome.success());

    SECTION("Put 4MB page successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 0, iss.str().size(), iss).get();
        REQUIRE(put_page_from_stream_outcome.success());

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 64 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str());
    }

    SECTION("Put 5MB page unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(5 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 0, iss.str().size(), iss).get();
        REQUIRE(!put_page_from_stream_outcome.success());
    }

    SECTION("Put 4MB page to an 512 byte aligned offset successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 1024, iss.str().size(), iss).get();
        REQUIRE(put_page_from_stream_outcome.success());

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 64 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 1024, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str());
    }

    SECTION("Put 4MB page to an 512 not aligned offset unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 511, iss.str().size(), iss).get();
        REQUIRE(!put_page_from_stream_outcome.success());
    }

    SECTION("Put 4MB page to same pages overwrites the data successfully")
    {
        auto iss1 = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 1024, iss1.str().size(), iss1).get();
        REQUIRE(put_page_from_stream_outcome.success());

        auto iss2 = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 1024, iss2.str().size(), iss2).get();
        REQUIRE(put_page_from_stream_outcome.success());

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 64 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 1024, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss2.str());
    }

    SECTION("Put 4MB page exceeds the size of the page blob unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 62 * 1024 * 1024, iss.str().size(), iss).get();
        REQUIRE(!put_page_from_stream_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Clear page", "[page blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, 4 * 1024 * 1024).get();
    REQUIRE(create_page_blob_outcome.success());
    auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
    auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, 0, iss.str().size(), iss).get();
    REQUIRE(put_page_from_stream_outcome.success());

    SECTION("Clear 4MB page successfully")
    {
        auto clear_page_outcome = client.clear_page(container_name, blob_name, 0, 4 * 1024 * 1024).get();
        REQUIRE(clear_page_outcome.success());

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        std::string all_0_string;
        all_0_string.append(4 * 1024 * 1024, '\0');
        REQUIRE(strbuf.str() == all_0_string);
    }

    SECTION("Clear 5MB page unsuccessfully")
    {
        auto clear_page_outcome = client.clear_page(container_name, blob_name, 0, 5 * 1024 * 1024).get();
        REQUIRE(!clear_page_outcome.success());
    }

    SECTION("Clear 1MB page to an 512 byte aligned offset successfully")
    {
        auto clear_page_outcome = client.clear_page(container_name, blob_name, 3 * 1024 * 1024, 1 * 1024 * 1024).get();
        REQUIRE(clear_page_outcome.success());

        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024);

        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str().substr(0, 3 * 1024 * 1024).append(1024 * 1024, '\0'));
    }

    SECTION("Clear 1MB page to an 512 not aligned offset unsuccessfully")
    {
        auto clear_page_outcome = client.clear_page(container_name, blob_name, 511, 1 * 1024 * 1024).get();
        REQUIRE(!clear_page_outcome.success());
    }

    SECTION("Clear 2MB page exceeds the size of the page blob unsuccessfully")
    {
        auto clear_page_outcome = client.clear_page(container_name, blob_name, 3 * 1024 * 1024, 2 * 1024 * 1024).get();
        REQUIRE(!clear_page_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Get page ranges", "[page blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);
    auto create_page_blob_outcome = client.create_page_blob(container_name, blob_name, 1024 * 20).get();
    REQUIRE(create_page_blob_outcome.success());
    for (unsigned i = 0; i < 10; ++i)
    {
        auto iss = as_test::get_istringstream_with_random_buffer(1024);
        auto put_page_from_stream_outcome = client.put_page_from_stream(container_name, blob_name, i * 1024 * 2, iss.str().size(), iss).get();
        REQUIRE(put_page_from_stream_outcome.success());
    }


    SECTION("Get all page ranges successfully")
    {
        auto get_page_range_outcome = client.get_page_ranges(container_name, blob_name, 0, 1024 * 20).get();
        REQUIRE(get_page_range_outcome.success());
        auto page_list = get_page_range_outcome.response().pagelist;
        for (unsigned i = 0; i < 10; ++i)
        {
            REQUIRE(page_list[i].start == i * 2 * 1024);
            REQUIRE(page_list[i].end == (i * 2 + 1) * 1024 - 1);
        }
    }

    SECTION("Get all page not 512 align successfully")
    {
        auto get_page_range_outcome = client.get_page_ranges(container_name, blob_name, 513, 4096).get();
        REQUIRE(get_page_range_outcome.success());
        auto page_list = get_page_range_outcome.response().pagelist;
        REQUIRE(page_list[0].start == 512);
        REQUIRE(page_list[0].end == 1023);
        REQUIRE(page_list[1].start == 2048);
        REQUIRE(page_list[1].end == 3071);
        REQUIRE(page_list[2].start == 4096);
        REQUIRE(page_list[2].end == 5119);
    }

    SECTION("Get page ranges with offset larger than page blob size unsuccessfully")
    {
        auto get_page_range_outcome = client.get_page_ranges(container_name, blob_name, 1024 * 21, 1024 * 20).get();
        REQUIRE(!get_page_range_outcome.success());
        REQUIRE(get_page_range_outcome.error().code == "416");
        REQUIRE(get_page_range_outcome.error().code_name == "InvalidRange");
    }

    SECTION("Get page ranges with size larger than page blob size successfully")
    {
        auto get_page_range_outcome = client.get_page_ranges(container_name, blob_name, 0, 1024 * 25).get();
        REQUIRE(get_page_range_outcome.success());
        auto page_list = get_page_range_outcome.response().pagelist;
        for (unsigned i = 0; i < 10; ++i)
        {
            REQUIRE(page_list[i].start == i * 2 * 1024);
            REQUIRE(page_list[i].end == (i * 2 + 1) * 1024 - 1);
        }
    }

    client.delete_container(container_name);
}
