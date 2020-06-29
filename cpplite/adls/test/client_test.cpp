#include "catch2/catch.hpp"

#include "adls_client.h"
#include "adls_test_base.h"

TEST_CASE("Client General", "[adls][client]")
{
    for (bool exception_enabled : {true, false})
    {
        azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(exception_enabled);
        REQUIRE(client.exception_enabled() == exception_enabled);
    }
}

TEST_CASE("Client Throw Exception", "[adls][client]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(true);

    bool exception_caught = false;
    try
    {
        client.delete_filesystem(as_test::get_random_string(10));
    }
    catch (const std::exception&)
    {
        exception_caught = true;
    }
    REQUIRE(exception_caught);
}

TEST_CASE("Client Errno", "[adls][client]")
{
    azure::storage_adls::adls_client client = as_test::adls_base::test_adls_client(false);

    bool exception_caught = false;
    int error_code = 0;
    try
    {
        client.delete_filesystem(as_test::get_random_string(10));
        error_code = errno;
    }
    catch (const std::exception&)
    {
        exception_caught = true;
    }
    REQUIRE_FALSE(exception_caught);
    REQUIRE(error_code != 0);
}