#include "blob_integration_base.h"
#include "storage_errno.h"
#include "mstream.h"

#include "catch2/catch.hpp"

TEST_CASE("Upload block blob from stream", "[block blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Upload block blob from a 2048 byte stream successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(2048);
        auto create_blob_outcome = client.upload_block_blob_from_stream(container_name, blob_name, iss, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(create_blob_outcome.success());
        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 4096, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str());
    }

    SECTION("Upload block blob from a 64MB stream successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(64 * 1024 * 1024);
        auto create_blob_outcome = client.upload_block_blob_from_stream(container_name, blob_name, iss, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(create_blob_outcome.success());
        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, 64 * 1024 * 1024, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == iss.str());
    }

    SECTION("Upload block blob from a 257MB stream unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(257 * 1024 * 1024);
        auto create_blob_outcome = client.upload_block_blob_from_stream(container_name, blob_name, iss, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(!create_blob_outcome.success());
    }

    SECTION("Upload block blob from with metadata successfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(2048);
        std::vector<std::pair<std::string, std::string>> meta;
        meta.push_back(std::make_pair(std::string("custommeta1"), std::string("meta1")));
        meta.push_back(std::make_pair(std::string("custommeta2"), std::string("meta2")));
        auto create_blob_outcome = client.upload_block_blob_from_stream(container_name, blob_name, iss, meta).get();
        REQUIRE(create_blob_outcome.success());
        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        for ( auto m : get_blob_property_outcome.response().metadata )
        {
            if (m.first == "custommeta1")
            {
                REQUIRE(m.second == "meta1");
            }
            else
            {
                REQUIRE(m.first == "custommeta2");
                REQUIRE(m.second == "meta2");

            }
        }
    }

    client.delete_container(container_name);
}

TEST_CASE("Upload blob block from stream", "[block blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Upload block from stream successfully")
    {
        for (unsigned i = 0; i < 10; ++i)
        {
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            std::string block_id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, block_id, iss).get();
            REQUIRE(upload_block_outcome.success());
        }
    }

    SECTION("Upload block from buffer successfully")
    {
        size_t block_size = 4 * 1024 * 1024;
        unsigned block_count = 10;
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        std::string full_content;
        for (unsigned i = 0; i < block_count; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto buff = as_test::get_random_buffer(block_size);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_buffer(container_name, blob_name, item.id, buff, block_size).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            full_content.append(buff, block_size);
            block_list.push_back(item);
            delete[] buff;
        }

        auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(put_block_list_outcome.success());
        std::stringbuf strbuf;
        std::ostream os(&strbuf);
        auto get_blob_outcome = client.download_blob_to_stream(container_name, blob_name, 0, block_size * block_count, os).get();
        REQUIRE(get_blob_outcome.success());
        REQUIRE(strbuf.str() == full_content);
    }

    SECTION("Upload block from stream with invalid block ID unsuccessfully")
    {
        auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
        std::string block_id = "000001";
        auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, block_id, iss).get();
        REQUIRE(!upload_block_outcome.success());
    }
    
    client.delete_container(container_name);
}

TEST_CASE("Put block list", "[block blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Put block list with all blocks uncommitted successfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }
        
        auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(put_block_list_outcome.success());
        auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
        REQUIRE(get_block_list_outcome.success());
        auto committed_block_list = get_block_list_outcome.response().committed;
        auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
        REQUIRE(committed_block_list.size() == 10);
        REQUIRE(uncommitted_block_list.size() == 0);
        auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
        REQUIRE(get_blob_property_outcome.success());
        REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 10);
    }

    SECTION("Put block list with all blocks committed successfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }
        {
            auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
            REQUIRE(put_block_list_outcome.success());
            auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
            REQUIRE(get_block_list_outcome.success());
            auto committed_block_list = get_block_list_outcome.response().committed;
            auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
            REQUIRE(committed_block_list.size() == 10);
            REQUIRE(uncommitted_block_list.size() == 0);
            auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
            REQUIRE(get_blob_property_outcome.success());
            REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 10);
        }
        for (unsigned i = 0; i < 10; ++i)
        {
            block_list[i].type = azure::storage_lite::put_block_list_request_base::block_type::committed;
        }
        {
            block_list.pop_back();
            auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
            REQUIRE(put_block_list_outcome.success());
            auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
            REQUIRE(get_block_list_outcome.success());
            auto committed_block_list = get_block_list_outcome.response().committed;
            auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
            REQUIRE(committed_block_list.size() == 9);
            REQUIRE(uncommitted_block_list.size() == 0);

            auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
            REQUIRE(get_blob_property_outcome.success());
            REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 9);
        }
    }

    SECTION("Put block list with both committed and uncommitted blocks successfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 5; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }
        {
            auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
            REQUIRE(put_block_list_outcome.success());
            auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
            REQUIRE(get_block_list_outcome.success());
            auto committed_block_list = get_block_list_outcome.response().committed;
            auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
            REQUIRE(committed_block_list.size() == 5);
            REQUIRE(uncommitted_block_list.size() == 0);
            auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
            REQUIRE(get_blob_property_outcome.success());
            REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 5);
        }
        for (unsigned i = 0; i < 5; ++i)
        {
            block_list[i].type = azure::storage_lite::put_block_list_request_base::block_type::committed;
        }
        block_list.pop_back();
        for (unsigned i = 5; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }
        {
            auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
            REQUIRE(put_block_list_outcome.success());
            auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
            REQUIRE(get_block_list_outcome.success());
            auto committed_block_list = get_block_list_outcome.response().committed;
            auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
            REQUIRE(committed_block_list.size() == 9);
            REQUIRE(uncommitted_block_list.size() == 0);

            auto get_blob_property_outcome = client.get_blob_properties(container_name, blob_name).get();
            REQUIRE(get_blob_property_outcome.success());
            REQUIRE(get_blob_property_outcome.response().size == 4 * 1024 * 1024 * 9);
        }
    }

    SECTION("Put block list with invalid block list unsuccessfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::committed;
            block_list.push_back(item);
        }

        auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(!put_block_list_outcome.success());
    }

    client.delete_container(container_name);
}

TEST_CASE("Get block list", "[block blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    SECTION("Get committed block list successfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }

        auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(put_block_list_outcome.success());

        auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
        REQUIRE(get_block_list_outcome.success());
        auto committed_block_list = get_block_list_outcome.response().committed;
        auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
        REQUIRE(committed_block_list.size() == 10);
        REQUIRE(uncommitted_block_list.size() == 0);
    }

    SECTION("Get un-committed block list successfully")
    {
        for (unsigned i = 0; i < 10; ++i)
        {
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            std::string block_id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, block_id, iss).get();
            REQUIRE(upload_block_outcome.success());
        }

        auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
        REQUIRE(get_block_list_outcome.success());
        auto committed_block_list = get_block_list_outcome.response().committed;
        auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
        REQUIRE(committed_block_list.size() == 0);
        REQUIRE(uncommitted_block_list.size() == 10);
    }

    SECTION("Get both committed and uncommitted block list successfully")
    {
        std::vector<azure::storage_lite::put_block_list_request_base::block_item> block_list;
        for (unsigned i = 0; i < 10; ++i)
        {
            azure::storage_lite::put_block_list_request_base::block_item item;
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            item.id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, item.id, iss).get();
            REQUIRE(upload_block_outcome.success());
            item.type = azure::storage_lite::put_block_list_request_base::block_type::uncommitted;
            block_list.push_back(item);
        }

        auto put_block_list_outcome = client.put_block_list(container_name, blob_name, block_list, std::vector<std::pair<std::string, std::string>>()).get();
        REQUIRE(put_block_list_outcome.success());

        for (unsigned i = 0; i < 10; ++i)
        {
            auto iss = as_test::get_istringstream_with_random_buffer(4 * 1024 * 1024);
            auto block_id = as_test::get_base64_block_id(i);
            auto upload_block_outcome = client.upload_block_from_stream(container_name, blob_name, block_id, iss).get();
            REQUIRE(upload_block_outcome.success());
        }

        auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
        REQUIRE(get_block_list_outcome.success());
        auto committed_block_list = get_block_list_outcome.response().committed;
        auto uncommitted_block_list = get_block_list_outcome.response().uncommitted;
        REQUIRE(committed_block_list.size() == 10);
        REQUIRE(uncommitted_block_list.size() == 10);
    }

    SECTION("Get empty block list successfully")
    {
        auto get_block_list_outcome = client.get_block_list(container_name, blob_name).get();
        REQUIRE(!get_block_list_outcome.success());
        REQUIRE(get_block_list_outcome.error().code == "404");
        REQUIRE(get_block_list_outcome.error().code_name == "BlobNotFound");
    }

    client.delete_container(container_name);
}

TEST_CASE("memory streambuf", "")
{
    if (sizeof(void*) == 8)
    {
        char* buffer = nullptr;
        uint64_t buffer_size = 16 * 1024 * 1024 * 1024ULL;
        azure::storage_lite::memory_streambuf buf(buffer, buffer_size);
        CHECK(buffer_size == buf.in_avail());
        int64_t size_3g = 3 * 1024 * 1024 * 1024ULL;
        buf.pubseekpos(size_3g);
        CHECK(buffer_size - size_3g == buf.in_avail());
        buf.pubseekoff(-size_3g, std::ios_base::end);
        CHECK(size_3g == buf.in_avail());
        buf.pubseekoff(size_3g, std::ios_base::beg);
        CHECK(buffer_size - size_3g == buf.in_avail());
        buf.pubseekoff(size_3g, std::ios_base::cur);
        CHECK(buffer_size - 2 * size_3g == buf.in_avail());
    }

    size_t size_10m = 10 * 1024 * 1024;
    char* buffer = as_test::get_random_buffer(size_10m);
    azure::storage_lite::memory_streambuf buf(buffer, size_10m);
    CHECK(size_10m == buf.in_avail());
    uint64_t offset = 0;
    CHECK(int(buffer[offset + 1]) == buf.snextc());
    ++offset;
    CHECK(int(buffer[offset + 1]) == buf.snextc());
    ++offset;
    CHECK(int(buffer[offset]) == buf.sbumpc());
    ++offset;
    CHECK(int(buffer[offset]) == buf.sbumpc());
    ++offset;
    CHECK(int(buffer[offset]) == buf.sgetc());
    CHECK(int(buffer[offset]) == buf.sgetc());

    buf.sputc('a');
    offset++;
    CHECK('a' == buffer[offset - 1]);
    buf.sputc('b');
    offset++;
    CHECK('b' == buffer[offset - 1]);

    size_t size_2m = 2 * 1024 * 1024;
    char* buffer2 = as_test::get_random_buffer(size_2m);
    buf.pubseekpos(size_2m);
    offset = size_2m;
    CHECK(size_2m == buf.sputn(buffer2, size_2m));
    offset += size_2m;
    CHECK(0 == std::memcmp(buffer2, buffer + offset - size_2m, size_2m));
    CHECK(size_2m == buf.sgetn(buffer2, size_2m));
    offset += size_2m;
    CHECK(0 == std::memcmp(buffer2, buffer + offset - size_2m, size_2m));

    int64_t size_512k = 512 * 1024;
    buf.pubseekoff(-size_512k, std::ios_base::end);
    CHECK(size_512k == buf.sgetn(buffer2, size_2m));
    CHECK(0 == buf.in_avail());
    buf.pubseekoff(-size_512k, std::ios_base::end);
    CHECK(size_512k == buf.sputn(buffer2, size_2m));
    CHECK(0 == buf.in_avail());

    delete[] buffer;
    delete[] buffer2;
}

TEST_CASE("memory stream", "")
{
    size_t buffer_size = 20 * 1024;
    size_t op_off = 3 * 1024 + 12345;
    size_t op_size1 = 1 * 1024 + 123;
    size_t op_size2 = 3 * 1024 + 456;

    {
        char* buffer = as_test::get_random_buffer(buffer_size);
        char* buffer2 = as_test::get_random_buffer(buffer_size);
        azure::storage_lite::imstream im(buffer, buffer_size);
        im.seekg(op_off);
        CHECK(op_off == im.tellg());
        im.read(buffer2, op_size1);
        CHECK(im.good());
        CHECK(op_size1 == im.gcount());
        CHECK(op_off + op_size1 == im.tellg());
        im.read(buffer2 + op_size1, op_size2);
        CHECK(im.good());
        CHECK(op_size2 == im.gcount());
        CHECK(op_off + op_size1 + op_size2 == im.tellg());
        CHECK(0 == std::memcmp(buffer + op_off, buffer2, op_size1 + op_size2));
        im.seekg(0, std::ios_base::end);
        CHECK(buffer_size == im.tellg());
        delete[] buffer;
        delete[] buffer2;
    }

    {
        char* buffer = as_test::get_random_buffer(buffer_size);
        char* buffer2 = as_test::get_random_buffer(buffer_size);
        azure::storage_lite::omstream om(buffer, buffer_size);
        om.seekp(op_off);
        CHECK(op_off == om.tellp());
        om.write(buffer2, op_size1);
        CHECK(om.good());
        CHECK(op_off + op_size1 == om.tellp());
        om.write(buffer2 + op_size1, op_size2);
        CHECK(om.good());
        CHECK(op_off + op_size1 + op_size2 == om.tellp());
        CHECK(0 == std::memcmp(buffer + op_off, buffer2, op_size1 + op_size2));
        om.seekp(0, std::ios_base::end);
        CHECK(buffer_size == om.tellp());
        delete[] buffer;
        delete[] buffer2;
    }
    {
        char* buffer = as_test::get_random_buffer(buffer_size);
        char* buffer2 = as_test::get_random_buffer(buffer_size);
        azure::storage_lite::mstream iom(buffer, buffer_size);
        iom.seekg(op_off);
        CHECK(op_off == iom.tellg());
        iom.read(buffer2 + op_off, op_size1);
        CHECK(iom.good());
        CHECK(op_size1 == iom.gcount());
        iom.seekp(op_off + op_size1);
        CHECK(op_off + op_size1 == iom.tellp());
        iom.write(buffer2 + op_off + op_size1, op_size2);
        CHECK(iom.good());
        CHECK(op_off + op_size1 + op_size2 == iom.tellp());
        CHECK(0 == std::memcmp(buffer + op_off, buffer2 + op_off, op_size1 + op_size2));
        iom.seekg(0, std::ios_base::end);
        CHECK(buffer_size == iom.tellg());
        iom.seekp(0, std::ios_base::end);
        CHECK(buffer_size == iom.tellp());
        delete[] buffer;
        delete[] buffer2;
    }
}

TEST_CASE("Parallel upload download", "[block blob],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client(16);
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    uint64_t max_block_blob_size = azure::storage_lite::constants::max_block_size * azure::storage_lite::constants::max_num_blocks;
    auto res = client.upload_block_blob_from_buffer(container_name, blob_name, nullptr, {}, max_block_blob_size + 1, 1).get();
    CHECK(!res.success());
    CHECK(res.error().code == std::to_string(blob_too_big));

    size_t blob_size = 64 * 1024 * 1024 + 1234;
    char* buffer = as_test::get_random_buffer(blob_size);
    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("meta_key1", "meta-key2"));
    res = client.upload_block_blob_from_buffer(container_name, blob_name, buffer, metadata, blob_size, 8).get();
    CHECK(res.success());
    auto properties = client.get_blob_properties(container_name, blob_name).get().response();
    CHECK(blob_size == properties.size);
    CHECK(metadata == properties.metadata);

    char* download_buffer = new char[blob_size];
    struct test_case
    {
        uint64_t offset;
        uint64_t size;
        int parallelism;
    };

    std::vector<test_case> test_cases
    {
        {0, blob_size, 1},
        {0, blob_size, 2},
        {0, blob_size, 4},
        {0, blob_size, 16},
        {123, blob_size - 123, 8},
        {33 * 1024 * 1024 + 1234, 8 * 1024 * 1024 + 5678, 8},
    };

    int i = 0;
    for (auto t : test_cases)
    {
        const char guard_c = '\xdd';
        std::memset(download_buffer, guard_c, blob_size);
        t.size = std::min(t.size, blob_size - t.offset);
        auto res = client.download_blob_to_buffer(container_name, blob_name, t.offset, t.size, download_buffer + t.offset, t.parallelism).get();
        CHECK(res.success());

        bool sane = true;
        for (size_t i = 0; i < blob_size && sane; ++i)
        {
            if (i >= t.offset && i < t.offset + t.size)
                sane &= buffer[i] == download_buffer[i];
            else
                sane &= guard_c == download_buffer[i];

        }
        CHECK(sane);
    }

    delete[] buffer;
    delete[] download_buffer;
    client.delete_container(container_name);
}

TEST_CASE("Parallel upload download benchmark", "[!hide][benchmark]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client(50);
    client.context()->set_retry_policy(std::make_shared<azure::storage_lite::no_retry_policy>());
    std::string container_name = as_test::create_random_container("", client);
    std::string blob_name = as_test::get_random_string(20);

    uint64_t blob_size = 4 * 1024 * 1024 * 1024ULL;
    char* buffer = new char[blob_size];
    for (char* p = buffer; p < buffer + blob_size; p += 4096)
    {
        *p = 0;
    }

    auto timer_start = std::chrono::steady_clock::now();
    auto res = client.upload_block_blob_from_buffer(container_name, blob_name, buffer, {}, blob_size, 50).get();
    CHECK(res.success());
    auto timer_end = std::chrono::steady_clock::now();
    double time_us = double(std::chrono::duration_cast<std::chrono::microseconds>(timer_end - timer_start).count());
    double speed = (blob_size / 1024 / 1024) / (time_us / 1e6);
    std::cout << "Upload:\n";
    std::cout << time_us / 1000 << "ms" << std::endl;
    std::cout << speed / 1024 * 8 << "Gbps" << std::endl;

    timer_start = std::chrono::steady_clock::now();
    res = client.download_blob_to_buffer(container_name, blob_name, 0, blob_size, buffer, 50).get();
    timer_end = std::chrono::steady_clock::now();
    time_us = double(std::chrono::duration_cast<std::chrono::microseconds>(timer_end - timer_start).count());
    speed = (blob_size / 1024 / 1024) / (time_us / 1e6);
    std::cout << "Download:\n";
    std::cout << time_us / 1000 << "ms" << std::endl;
    std::cout << speed / 1024 * 8 << "Gbps" << std::endl;

    delete[] buffer;
    client.delete_container(container_name);
}
