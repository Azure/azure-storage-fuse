#include "blob_integration_base.h"

#include "catch2/catch.hpp"

TEST_CASE("Create containers", "[container],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string prefix = as_test::get_random_string(10);
    SECTION("Create container with number and character name successfully")
    {
        auto container_name = prefix + as_test::get_random_string(10);
        auto first_outcome = client.create_container(container_name).get();
        REQUIRE(first_outcome.success());
        auto second_outcome = client.get_container_properties(container_name).get();
        REQUIRE(second_outcome.success());
        REQUIRE_FALSE(second_outcome.response().etag.empty());
        client.delete_container(container_name).wait();
    }

    SECTION("Create container with uppercase name unsuccessfully")
    {
        auto container_name = "ABD" + prefix + as_test::get_random_string(10);
        auto first_outcome = client.create_container(container_name).get();
        REQUIRE_FALSE(first_outcome.success());
        REQUIRE(first_outcome.error().code == "400");
        REQUIRE(first_outcome.error().code_name == "InvalidResourceName");
        auto second_outcome = client.get_container_properties(container_name).get();
        REQUIRE_FALSE(second_outcome.success());
        REQUIRE(second_outcome.response().etag.empty());
    }

    SECTION("Create container with dash in name successfully")
    {
        auto container_name = prefix + "-" + as_test::get_random_string(10);
        auto first_outcome = client.create_container(container_name).get();
        REQUIRE(first_outcome.success());
        auto second_outcome = client.get_container_properties(container_name).get();
        REQUIRE(second_outcome.success());
        REQUIRE_FALSE(second_outcome.response().etag.empty());
        client.delete_container(container_name).wait();
    }
}

TEST_CASE("Delete containers", "[container],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string prefix = as_test::get_random_string(10);
    SECTION("Delete existing container successfully")
    {
        auto container_name = prefix + as_test::get_random_string(10);
        auto first_outcome = client.create_container(container_name).get();
        REQUIRE(first_outcome.success());
        auto second_outcome = client.get_container_properties(container_name).get();
        REQUIRE(second_outcome.success());
        REQUIRE_FALSE(second_outcome.response().etag.empty());
        auto third_outcome = client.delete_container(container_name).get();
        REQUIRE(third_outcome.success());
    }

    SECTION("Delete in-existing container successfully")
    {
        auto container_name = prefix + as_test::get_random_string(10);
        auto first_outcome = client.delete_container(container_name).get();
        REQUIRE_FALSE(first_outcome.success());
    }
}

TEST_CASE("Get Container Property", "[container], [blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string prefix = as_test::get_random_string(10);
    SECTION("Get container property from existing container")
    {
        auto container_name = prefix + as_test::get_random_string(10);
        auto first_outcome = client.create_container(container_name).get();
        REQUIRE(first_outcome.success());
        auto second_outcome = client.get_container_properties(container_name).get();
        REQUIRE(second_outcome.success());
        REQUIRE_FALSE(second_outcome.response().etag.empty());

        std::vector<std::pair<std::string, std::string>> metadata;
        metadata.emplace_back(std::make_pair("mkey1", "mvalue1"));
        metadata.emplace_back(std::make_pair("mkEy2", "MValUe2#  % %2D"));
        auto third_outcome = client.set_container_metadata(container_name, metadata).get();
        REQUIRE(third_outcome.success());

        auto fourth_outcome = client.get_container_properties(container_name).get();
        REQUIRE(fourth_outcome.success());
        REQUIRE(fourth_outcome.response().metadata == metadata);

        metadata.clear();
        auto fifth_outcome = client.set_container_metadata(container_name, metadata).get();
        REQUIRE(fifth_outcome.success());

        auto sixth_outcome = client.get_container_properties(container_name).get();
        REQUIRE(sixth_outcome.success());
        REQUIRE(sixth_outcome.response().metadata == metadata);

        client.delete_container(container_name).wait();
    }

    SECTION("Get container property from non-existing container")
    {
        auto container_name = prefix + as_test::get_random_string(10);
        auto first_outcome = client.get_container_properties(container_name).get();
        REQUIRE_FALSE(first_outcome.success());
    }
}

TEST_CASE("List containers segmented", "[container],[blob_service]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();
    std::string prefix_1 = as_test::get_random_string(10);
    std::string prefix_2 = as_test::get_random_string(10);
    unsigned container_size = 5;
    std::vector<std::string> containers;
    for (unsigned i = 0; i < container_size; ++i)
    {
        auto container_name_1 = prefix_1 + as_test::get_random_string(10);
        auto container_name_2 = prefix_2 + as_test::get_random_string(10);
        client.create_container(container_name_1).wait();
        client.create_container(container_name_2).wait();
        containers.push_back(container_name_1);
        containers.push_back(container_name_2);
    }

    SECTION("List containers successfully") {
        auto list_containers_outcome = client.list_containers_segmented("", "", 10).get();
        REQUIRE(list_containers_outcome.success());
        auto result_containers = list_containers_outcome.response().containers;
        REQUIRE(result_containers.size() == 10);
    }

    SECTION("List containers with prefix successfully")
    {
        {
            auto list_containers_outcome = client.list_containers_segmented(prefix_1, "", 5).get();
            REQUIRE(list_containers_outcome.success());
            REQUIRE(list_containers_outcome.response().next_marker.empty());
            auto result_containers = list_containers_outcome.response().containers;
            REQUIRE(result_containers.size() == 5);
            for (auto container : result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_1.size()) == prefix_1);
            }
        }

        {
            auto list_containers_outcome = client.list_containers_segmented(prefix_2, "", 5).get();
            REQUIRE(list_containers_outcome.success());
            REQUIRE(list_containers_outcome.response().next_marker.empty());
            auto result_containers = list_containers_outcome.response().containers;
            REQUIRE(result_containers.size() == 5);
            for (auto container : result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_2.size()) == prefix_2);
            }
        }

        {
            auto list_containers_outcome = client.list_containers_segmented(as_test::get_random_string(20), "").get();
            REQUIRE(list_containers_outcome.success());
            REQUIRE(list_containers_outcome.response().containers.size() == 0);
        }
    }

    SECTION("List containers with next marker successfully")
    {
        {
            auto list_containers_outcome = client.list_containers_segmented(prefix_1, "", 3).get();
            REQUIRE(list_containers_outcome.success());
            REQUIRE(!list_containers_outcome.response().next_marker.empty());
            auto result_containers = list_containers_outcome.response().containers;
            REQUIRE(result_containers.size() == 3);
            for (auto container : result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_1.size()) == prefix_1);
            }

            auto second_list_containers_outcome = client.list_containers_segmented(prefix_1, list_containers_outcome.response().next_marker, 2).get();
            REQUIRE(second_list_containers_outcome.response().next_marker.empty());
            auto second_result_containers = second_list_containers_outcome.response().containers;
            REQUIRE(second_result_containers.size() == 2);
            for (auto container : second_result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_1.size()) == prefix_1);
            }
        }

        {
            auto list_containers_outcome = client.list_containers_segmented(prefix_2, "", 3).get();
            REQUIRE(list_containers_outcome.success());
            REQUIRE(!list_containers_outcome.response().next_marker.empty());
            auto result_containers = list_containers_outcome.response().containers;
            REQUIRE(result_containers.size() == 3);
            for (auto container : result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_1.size()) == prefix_2);
            }

            auto second_list_containers_outcome = client.list_containers_segmented(prefix_2, list_containers_outcome.response().next_marker, 2).get();
            REQUIRE(second_list_containers_outcome.response().next_marker.empty());
            auto second_result_containers = second_list_containers_outcome.response().containers;
            REQUIRE(second_result_containers.size() == 2);
            for (auto container : second_result_containers)
            {
                REQUIRE(std::find(containers.begin(), containers.end(), container.name) != containers.end());
                REQUIRE(container.name.substr(0, prefix_1.size()) == prefix_2);
            }
        }
    }

    SECTION("List containers with invalid prefix successfully")
    {
        auto list_containers_outcome = client.list_containers_segmented("1~invalid~~%d_prefix", "").get();
        REQUIRE(list_containers_outcome.success());
        REQUIRE(list_containers_outcome.response().containers.empty());
    }

    SECTION("List containers with invalid next marker unsuccessfully")
    {
        auto list_containers_outcome = client.list_containers_segmented("", "1~invalid~~%d_continuation_token").get();
        REQUIRE(!list_containers_outcome.success());
        REQUIRE(list_containers_outcome.error().code == "400");
        REQUIRE(list_containers_outcome.error().code_name == "OutOfRangeInput");
    }

    for (auto container : containers)
    {
        client.delete_container(container).wait();
    }
}

TEST_CASE("SAS Authorization", "[blob_service][sas]")
{
    std::string account_name = "";
    std::string sas_token = "";
    if (account_name.empty() || sas_token.empty())
    {
        return;
    }

    auto cred = std::make_shared<azure::storage_lite::shared_access_signature_credential>(sas_token);
    auto account = std::make_shared<azure::storage_lite::storage_account>(account_name, cred);
    auto client = std::make_shared<azure::storage_lite::blob_client>(account, 1);

    std::string container_name = as_test::get_random_string(10);
    auto outcome = client->create_container(container_name).get();
    REQUIRE(outcome.success());
    outcome = client->delete_container(container_name).get();
    REQUIRE(outcome.success());
}

TEST_CASE("Token Authorization", "[blob_service][token]")
{
    std::string account_name = "";
    std::string oauth_token = "";
    if (account_name.empty() || oauth_token.empty())
    {
        return;
    }

    auto cred = std::make_shared<azure::storage_lite::token_credential>(oauth_token);
    auto account = std::make_shared<azure::storage_lite::storage_account>(account_name, cred);
    auto client = std::make_shared<azure::storage_lite::blob_client>(account, 1);

    std::string container_name = as_test::get_random_string(10);
    auto outcome = client->create_container(container_name).get();
    REQUIRE(outcome.success());
    outcome = client->delete_container(container_name).get();
    REQUIRE(outcome.success());
}