#include "catch2/catch.hpp"

#include "adls_client.h"
#include "adls_test_base.h"

TEST_CASE("Create File", "[adls][file]")
{
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

        std::string fs_name = as_test::adls_base::create_random_filesystem(client);

        std::string file_name = as_test::get_random_string(10) + "/" + as_test::get_random_string(10);
        client.create_file(fs_name, file_name);
        REQUIRE(errno == 0);
        REQUIRE(client.file_exists(fs_name, file_name));

        client.delete_file(fs_name, file_name);
        REQUIRE(errno == 0);
        REQUIRE_FALSE(client.file_exists(fs_name, file_name));

        client.delete_file(fs_name, file_name);
        REQUIRE(errno != 0);

        client.delete_filesystem(fs_name);
    }
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(true);
        std::string fs_name = as_test::adls_base::create_random_filesystem(client);

        std::string file_name = as_test::get_random_string(10);
        REQUIRE_THROWS_AS(client.delete_file(fs_name, file_name), std::exception);
        REQUIRE_FALSE(client.file_exists(fs_name, file_name));
        REQUIRE(errno == 0);

        client.delete_filesystem(fs_name);
    }
}

TEST_CASE("Append Data", "[adls][file]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);

    std::string file_name = as_test::get_random_string(10);
    client.create_file(fs_name, file_name);

    std::mt19937_64 gen(std::random_device{}());
    std::uniform_int_distribution<uint64_t> dist(1, 8 * 1024 * 1024);

    std::vector<std::pair<uint64_t, uint64_t>> segments;
    for (int i = 0; i < 4; ++i)
    {
        uint64_t segment_size = dist(gen);
        segments.emplace_back(std::make_pair(segments.empty() ? uint64_t(0) : segments.back().first + segments.back().second, segment_size));
    }
    uint64_t file_size = segments.back().first + segments.back().second;
    std::shuffle(segments.begin(), segments.end(), gen);

    std::string file_content;
    file_content.resize(file_size);
    for (auto p : segments)
    {
        uint64_t offset = p.first;
        uint64_t segment_length = p.second;
        auto in_stream = as_test::get_istringstream_with_random_buffer(segment_length);
        in_stream.read(&file_content[offset], segment_length);
        in_stream.seekg(0);
        client.append_data_from_stream(fs_name, file_name, offset, in_stream);
        REQUIRE(errno == 0);
    }
    client.flush_data(fs_name, file_name, file_size);
    REQUIRE(errno == 0);

    // total download
    std::ostringstream out_stream;
    client.download_file_to_stream(fs_name, file_name, out_stream);
    REQUIRE(file_content == out_stream.str());

    // range download
    for (int i = 0; i < 10; ++i)
    {
        uint64_t random_offset = std::uniform_int_distribution<uint64_t>(0, file_size - 1)(gen);
        uint64_t random_length = std::uniform_int_distribution<uint64_t>(1, file_size - random_offset + 1)(gen);
        std::ostringstream out_stream;
        client.download_file_to_stream(fs_name, file_name, random_offset, random_length, out_stream);
        REQUIRE(file_content.substr(random_offset, random_length) == out_stream.str());
    }

    // download till end
    out_stream.str("");
    uint64_t random_offset = std::uniform_int_distribution<uint64_t>(0, file_size - 1)(gen);
    uint64_t random_length = file_size - random_offset;
    client.download_file_to_stream(fs_name, file_name, random_offset, 0, out_stream);
    REQUIRE(random_length == out_stream.str().length());
    REQUIRE(file_content.substr(random_offset, random_length) == out_stream.str());

    // download nothing
    out_stream.str("");
    client.download_file_to_stream(fs_name, file_name, file_size, 0, out_stream);
    REQUIRE(out_stream.str().length() == 0);

    client.delete_filesystem(fs_name);
}

TEST_CASE("Upload File", "[adls][file]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);

    std::string file_name = as_test::get_random_string(10);

    std::string file_content;
    size_t file_size = 4 * 1024 * 1024 + 1234;
    file_content.resize(file_size);
    auto in_stream = as_test::get_istringstream_with_random_buffer(file_size);
    in_stream.read(&file_content[0], file_size);
    in_stream.seekg(0);

    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("key1", "value1"));
    metadata.emplace_back(std::make_pair("keY2", "ValUe2"));

    client.upload_file_from_stream(fs_name, file_name, in_stream, metadata);
    REQUIRE(errno == 0);

    std::ostringstream out_stream;
    client.download_file_to_stream(fs_name, file_name, out_stream);
    REQUIRE(file_content == out_stream.str());

    auto metadata2 = client.get_file_properties(fs_name, file_name);
    REQUIRE(metadata == metadata2);

    client.delete_filesystem(fs_name);
}

TEST_CASE("File Access Control", "[adls][file][acl]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string file_name = as_test::get_random_string(10);
    client.create_file(fs_name, file_name);

    azure::storage_adls::access_control acl;
    acl.acl = "user::rw-,group::rw-,other::r--";

    client.set_file_access_control(fs_name, file_name, acl);
    auto acl2 = client.get_file_access_control(fs_name, file_name);
    REQUIRE(acl2.owner == "$superuser");
    REQUIRE(acl2.group == "$superuser");
    REQUIRE(acl2.acl == acl.acl);

    acl.acl.clear();
    acl.permissions = "0644";
    client.set_file_access_control(fs_name, file_name, acl);
    acl2 = client.get_file_access_control(fs_name, file_name);
    REQUIRE(acl2.owner == "$superuser");
    REQUIRE(acl2.group == "$superuser");
    REQUIRE(acl2.permissions == "rw-r--r--");
    client.delete_filesystem(fs_name);
}

TEST_CASE("Move File", "[adls][file]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs1_name = as_test::adls_base::create_random_filesystem(client);
    std::string fs2_name = as_test::adls_base::create_random_filesystem(client);

    std::string file1_name = as_test::get_random_string(10) + "/" + as_test::get_random_string(10);
    std::string file2_dir = as_test::get_random_string(10);
    std::string file2_basename = as_test::get_random_string(10);
    std::string file2_name = file2_dir + "/" + file2_basename;

    std::string file_content;
    size_t file_size = 512 * 1024;
    file_content.resize(file_size);
    auto in_stream = as_test::get_istringstream_with_random_buffer(file_size);
    in_stream.read(&file_content[0], file_size);
    in_stream.seekg(0);
    client.upload_file_from_stream(fs1_name, file1_name, in_stream);
    REQUIRE(errno == 0);

    client.create_directory(fs2_name, file2_dir);
    // Move file
    client.move_file(fs1_name, file1_name, fs2_name, file2_name);
    REQUIRE(errno == 0);
    REQUIRE(client.file_exists(fs2_name, file2_name));
    REQUIRE_FALSE(client.file_exists(fs1_name, file1_name));

    std::ostringstream out_stream;
    client.download_file_to_stream(fs2_name, file2_name, out_stream);
    REQUIRE(file_content == out_stream.str());

    std::string dest_dir = as_test::get_random_string(10);
    client.create_directory(fs2_name, dest_dir);
    // Move file into a directory
    client.move_file(fs2_name, file2_name, dest_dir);
    REQUIRE(errno == 0);
    REQUIRE_FALSE(client.file_exists(fs2_name, file2_name));
    REQUIRE(client.file_exists(fs2_name, dest_dir + "/" + file2_basename));

    client.delete_filesystem(fs1_name);
    client.delete_filesystem(fs2_name);
}

TEST_CASE("File Properties", "[adls][file]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string file_name = as_test::get_random_string(10);
    client.create_file(fs_name, file_name);

    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("mkey1", "mvalue1"));
    metadata.emplace_back(std::make_pair("mKey2", "mvAlue1    $%^&#"));
    client.set_file_properties(fs_name, file_name, metadata);
    REQUIRE(errno == 0);

    auto metadata2 = client.get_file_properties(fs_name, file_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    metadata.clear();
    client.set_file_properties(fs_name, file_name, metadata);
    REQUIRE(errno == 0);

    metadata2 = client.get_file_properties(fs_name, file_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    client.delete_filesystem(fs_name);
}