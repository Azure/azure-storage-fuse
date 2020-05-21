#include "catch2/catch.hpp"

#include "adls_client.h"
#include "adls_test_base.h"

TEST_CASE("Create Directory", "[adls][directory]")
{
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);
        std::string fs_name = as_test::adls_base::create_random_filesystem(client);

        std::string dir_name = as_test::get_random_string(10);
        std::string dir_name2 = dir_name + "/" + as_test::get_random_string(10);
        client.create_directory(fs_name, dir_name);
        REQUIRE(errno == 0);
        REQUIRE(client.directory_exists(fs_name, dir_name));

        client.create_directory(fs_name, dir_name2);
        REQUIRE(errno == 0);
        REQUIRE(client.directory_exists(fs_name, dir_name2));

        client.delete_directory(fs_name, dir_name);
        REQUIRE(errno == 0);
        REQUIRE_FALSE(client.directory_exists(fs_name, dir_name));
        REQUIRE_FALSE(client.directory_exists(fs_name, dir_name2));

        client.delete_filesystem(fs_name);
    }
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(true);
        std::string fs_name = as_test::adls_base::create_random_filesystem(client);

        std::string dir_name = as_test::get_random_string(10);
        REQUIRE_THROWS_AS(client.delete_directory(fs_name, dir_name), std::exception);
        REQUIRE_FALSE(client.directory_exists(fs_name, dir_name));
        REQUIRE(errno == 0);

        client.delete_filesystem(fs_name);
    }
}

TEST_CASE("Recursive Delete Directory", "[adls][directory]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);
    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string dir_name = as_test::get_random_string(10);
    client.create_directory(fs_name, dir_name);
    REQUIRE(client.directory_exists(fs_name, dir_name));

    int file_num = 100;

    std::vector<std::future<bool>> handles;
    for (int i = 0; i < file_num; ++i)
    {
        auto create_file_func = [fs_name, dir_name]()
        {
            azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);
            client.create_file(fs_name, dir_name + "/" + as_test::get_random_string(10));
            return errno == 0;
        };
        handles.emplace_back(std::async(std::launch::async, create_file_func));
    }
    for (auto& handle : handles)
    {
        REQUIRE(handle.get());
    }

    client.delete_directory(fs_name, dir_name);
    REQUIRE_FALSE(client.directory_exists(fs_name, dir_name));

    client.delete_filesystem(fs_name);
}

TEST_CASE("Directory Access Control", "[adls][directory][acl]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string dir_name = as_test::get_random_string(10);
    client.create_directory(fs_name, dir_name);

    azure::storage_adls::access_control acl;
    acl.acl = "user::rw-,group::rw-,other::r--";

    client.set_directory_access_control(fs_name, dir_name, acl);
    auto acl2 = client.get_directory_access_control(fs_name, dir_name);
    REQUIRE(acl2.owner == "$superuser");
    REQUIRE(acl2.group == "$superuser");
    REQUIRE(acl2.acl == acl.acl);

    acl.acl.clear();
    acl.permissions = "0644";
    client.set_directory_access_control(fs_name, dir_name, acl);
    acl2 = client.get_directory_access_control(fs_name, dir_name);
    REQUIRE(acl2.owner == "$superuser");
    REQUIRE(acl2.group == "$superuser");
    REQUIRE(acl2.permissions == "rw-r--r--");
    client.delete_filesystem(fs_name);
}

TEST_CASE("List Paths", "[adls][directory]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);
    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string dir_name = as_test::get_random_string(10);
    client.create_directory(fs_name, dir_name);

    std::set<std::string> recursive_paths;
    std::set<std::string> non_recursive_paths;
    for (int i = 0; i < 5; ++i)
    {
        std::string dir2_name = dir_name + "/" + as_test::get_random_string(10);
        client.create_directory(fs_name, dir2_name);
        for (int i = 0; i < 5; ++i)
        {
            std::string dir3_name = dir2_name + "/" + as_test::get_random_string(10);
            client.create_directory(fs_name, dir3_name);
            recursive_paths.emplace(dir3_name);
        }
        non_recursive_paths.emplace(dir2_name);
        recursive_paths.emplace(dir2_name);
    }

    for (auto do_recursive : { false, true })
    {
        std::string continuation;
        std::set<std::string> list_paths;
        do
        {
            auto list_result = client.list_paths_segmented(fs_name, dir_name, do_recursive, continuation, 5);
            for (auto& p : list_result.paths)
            {
                REQUIRE(!p.name.empty());
                REQUIRE(!p.etag.empty());
                REQUIRE(!p.last_modified.empty());
                REQUIRE(!p.acl.owner.empty());
                REQUIRE(!p.acl.group.empty());
                REQUIRE(!p.acl.permissions.empty());
                REQUIRE(p.acl.acl.empty());
                list_paths.emplace(p.name);
                REQUIRE(p.is_directory);
            }
            continuation = list_result.continuation_token;
        } while (!continuation.empty());
        REQUIRE(list_paths == (do_recursive ? recursive_paths : non_recursive_paths));
    }
    client.delete_filesystem(fs_name);
}

TEST_CASE("Move Directory", "[adls][directory]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs1_name = as_test::adls_base::create_random_filesystem(client);
    std::string fs2_name = as_test::adls_base::create_random_filesystem(client);

    std::string src_dir = as_test::get_random_string(10);
    std::string sub_dir1 = as_test::get_random_string(10);
    std::string file1 = sub_dir1 + "/" + as_test::get_random_string(10);
    std::string file2 = sub_dir1 + "/" + as_test::get_random_string(10) + "/" + as_test::get_random_string(10);
    std::string file3 = as_test::get_random_string(10);

    client.create_directory(fs1_name, src_dir);
    client.create_file(fs1_name, src_dir + "/" + file1);
    client.create_file(fs1_name, src_dir + "/" + file2);
    client.create_file(fs1_name, src_dir + "/" + file3);
    REQUIRE(errno == 0);

    std::string dest_dir = as_test::get_random_string(10);
    client.create_directory(fs2_name, dest_dir);
    // Move a directory into another directory
    client.move_directory(fs1_name, src_dir, fs2_name, dest_dir);
    REQUIRE(errno == 0);

    REQUIRE_FALSE(client.file_exists(fs1_name, src_dir + "/" + file1));
    REQUIRE_FALSE(client.file_exists(fs1_name, src_dir + "/" + file2));
    REQUIRE_FALSE(client.file_exists(fs1_name, src_dir + "/" + file3));
    REQUIRE(client.file_exists(fs2_name, dest_dir + "/" + src_dir + "/" + file1));
    REQUIRE(client.file_exists(fs2_name, dest_dir + "/" + src_dir + "/" + file2));
    REQUIRE(client.file_exists(fs2_name, dest_dir + "/" + src_dir + "/" + file3));

    client.create_directory(fs1_name, src_dir);
    file1 = as_test::get_random_string(10);
    client.create_file(fs1_name, file1);
    // Move directory onto a file
    client.move_directory(fs1_name, src_dir, file1);
    REQUIRE(errno != 0);

    client.delete_filesystem(fs1_name);
    client.delete_filesystem(fs2_name);
}

TEST_CASE("Directory Properties", "[adls][directory]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    std::string fs_name = as_test::adls_base::create_random_filesystem(client);
    std::string dir_name = as_test::get_random_string(10);
    client.create_directory(fs_name, dir_name);

    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("mkey1", "mvalue1"));
    metadata.emplace_back(std::make_pair("mKey2", "mvAlue1    $%^&#"));
    client.set_directory_properties(fs_name, dir_name, metadata);
    REQUIRE(errno == 0);

    auto metadata2 = client.get_directory_properties(fs_name, dir_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    metadata.clear();
    client.set_directory_properties(fs_name, dir_name, metadata);
    REQUIRE(errno == 0);

    metadata2 = client.get_directory_properties(fs_name, dir_name);
    REQUIRE(errno == 0);
    REQUIRE(metadata == metadata2);

    client.delete_filesystem(fs_name);
}

TEST_CASE("ADLS Token Authorization", "[adls][directory][token]")
{
    std::string account_name = "";
    std::string oauth_token = "";
    if (account_name.empty() || oauth_token.empty())
    {
        return;
    }

    auto cred = std::make_shared<azure::storage_lite::token_credential>(oauth_token);
    auto account = std::make_shared<azure::storage_lite::storage_account>(account_name, cred);
    auto client = std::make_shared<azure::storage_adls::adls_client>(account, 1);

    std::string fs_name = as_test::adls_base::create_random_filesystem(*client);
    std::string dir_name = as_test::get_random_string(10);
    client->create_directory(fs_name, dir_name);
    REQUIRE(errno == 0);

    client->delete_filesystem(fs_name);
}