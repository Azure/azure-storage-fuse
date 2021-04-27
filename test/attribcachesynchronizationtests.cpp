#include <uuid/uuid.h>
#include "gtest/gtest.h"
#include "gmock/gmock.h"
#include "blobfuse.h"

using ::testing::_;
using ::testing::Return;

class MockBlobClient : public blob_client_wrapper {
public:
    MockBlobClient() : blob_client_wrapper(true) {}
    MockBlobClient(std::shared_ptr<blob_client> &wrapper) : 
        blob_client_wrapper(wrapper) {}
       
    MOCK_CONST_METHOD0(is_valid, bool());
    MOCK_METHOD5(list_blobs_segmented, list_blobs_segmented_response(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults));
    MOCK_METHOD4(put_blob, void(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata));
    MOCK_METHOD5(upload_block_blob_from_stream, void(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata, size_t streamlen));
    MOCK_METHOD5(upload_file_to_blob, void(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata, size_t parallel));
    MOCK_METHOD5(download_blob_to_stream, void(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os));
    MOCK_METHOD5(download_blob_to_file, void(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel));
    MOCK_METHOD2(get_blob_property, blob_property(const std::string &container, const std::string &blob));
    MOCK_METHOD2(blob_exists, bool(const std::string &container, const std::string &blob));
    MOCK_METHOD2(delete_blob, void(const std::string &container, const std::string &blob));    
    MOCK_METHOD2(delete_blobdir, void(const std::string &container, const std::string &blob));
    MOCK_METHOD4(start_copy, void(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob));
};

// These tests validate that calls into the cache layer from multiple threads are synchronized & serialized properly.
// Correctness of return values is not tested (that's in another file.)
// The approach is, for every possible pair of operations (or calls), run the operations in parallel in three different scenarios:
//      - On the same blob
//      - On two different blobs in the same directory
//      - On two blobs in different directories.
// The first operation that runs contains a small delay; the second one does not.  If the operations should be serialized, the first must finish first; otherwise the second should finish first.
// Parameterized testing is used to test each pair of operations in a separate test.
class AttribCacheSynchronizationTest : public ::testing::TestWithParam<std::tuple<std::string, std::string, int>> {
public:
    void prep_mock(std::shared_ptr<std::mutex> m, std::shared_ptr<std::condition_variable> cv, std::shared_ptr<int> calls, std::shared_ptr<bool> sleep_finished);

    // Using a nice mock greatly simplifies testing, which is fine because we're not testing for cache correctness here.
    std::shared_ptr<::testing::NiceMock<MockBlobClient>> mockClient;
    std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper;
    std::string container_name;

    // This runs before each test.
    virtual void SetUp()
    {
       container_name = "container";
        #if 1
        mockClient = std::make_shared<::testing::NiceMock<MockBlobClient>>();
        #else
        int ret = read_config("../connection.cfg");
        ASSERT_EQ(0, ret) << "Read config failed.";
        std::string blob_endpoint;
        std::string sas_token;
        config_options.accountName.erase(remove(config_options.accountName.begin(), config_options.accountName.end(), '\r'), config_options.accountName.end());
        config_options.accountKey.erase(remove(config_options.accountKey.begin(), config_options.accountKey.end(), '\r'), config_options.accountKey.end());

        std::shared_ptr<storage_credential> cred;
        if (config_options.accountKey.length() > 0)
        {
            cred = std::make_shared<shared_key_credential>(config_options.accountName, config_options.accountKey);
            std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                    config_options.accountName, 
                    cred, 
                    true, 
                    blob_endpoint);
            // see if the code works with no cert
            std::string caCertFile;
            std::shared_ptr<blob_client> blobClient= std::make_shared<azure::storage_lite::blob_client>(account, 20, caCertFile);
            mockClient = std::make_shared<::testing::StrictMock<MockBlobClient>>(blobClient);
        }
        else
        {
            syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
            mockClient = std::make_shared<::testing::StrictMock<MockBlobClient>>();
        }
        #endif
        attrib_cache_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(mockClient);
    }

    virtual void TearDown()
    {
    }
};

// Mocked methods should call into this; it tracks the number of calls that have been made, and sleeps if this is the first call.
void prep(std::shared_ptr<std::mutex> m, std::shared_ptr<std::condition_variable> cv, std::shared_ptr<int> calls, std::shared_ptr<bool> sleep_finished)
{
    int call = 0;
    {
        std::lock_guard<std::mutex> lk(*m);
        call = *calls;
        (*calls)++;
    }

    if (call == 0)
    {
        (*cv).notify_one();
        std::this_thread::sleep_for(std::chrono::milliseconds(100)); // TODO: Consider making this value larger if tests are flaky.
        std::lock_guard<std::mutex> lk(*m);
        *sleep_finished = true;
    }
}

// Sets up a default action on every potential mocked method.
void AttribCacheSynchronizationTest::prep_mock(std::shared_ptr<std::mutex> m, std::shared_ptr<std::condition_variable> cv, std::shared_ptr<int> calls, std::shared_ptr<bool> sleep_finished)
{
    ON_CALL(*mockClient, get_blob_property(_, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
        return blob_property(true);
    }));

    ON_CALL(*mockClient, list_blobs_segmented(_, _ ,_, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
        return list_blobs_segmented_response();
    }));

    ON_CALL(*mockClient, put_blob(_, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, upload_block_blob_from_stream(_, _, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, upload_file_to_blob(_, _, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, download_blob_to_stream(_, _, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, download_blob_to_file(_, _, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, blob_exists(_, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
        return true;
    }));
    ON_CALL(*mockClient, delete_blob(_, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
    ON_CALL(*mockClient, start_copy(_, _, _, _))
    .WillByDefault(::testing::InvokeWithoutArgs([=] ()
    {
        prep(m, cv, calls, sleep_finished);
    }));
}

// This maps the operation name to the code required to run the actual operation.
// A promise is used in the event that the code is being called async.
std::map<std::string, std::function<void(std::shared_ptr<blob_client_attr_cache_wrapper>, std::string, std::string, std::promise<void>)>> fnMap = 
{
    {"Get", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            attrib_cache_wrapper->get_blob_property(container_name, blob);
            promise.set_value();
        }},
    {"Put", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->put_blob("source_path", container_name, blob, metadata);
            promise.set_value();
        }},
    {"UploadFromStream", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            std::stringstream is;
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->upload_block_blob_from_stream(container_name, blob, is, metadata);
            promise.set_value();
        }},
    {"UploadFromFile", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->upload_file_to_blob("source_path", container_name, blob, metadata, 10);
            promise.set_value();
        }},
    {"DownloadToStream", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            std::stringstream os;
            attrib_cache_wrapper->download_blob_to_stream(container_name, blob, 0, 10, os);
            promise.set_value();
        }},
    {"DownloadToFile", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            time_t lmt;;
            attrib_cache_wrapper->download_blob_to_file(container_name, blob, "dest_path", lmt, 10);
            promise.set_value();
        }},
    {"Exists", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            attrib_cache_wrapper->blob_exists(container_name, blob);
            promise.set_value();
        }},
    {"Delete", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            attrib_cache_wrapper->delete_blob(container_name, blob);
            promise.set_value();
        }},
    {"Copy", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob, std::promise<void> promise)
        {
            attrib_cache_wrapper->start_copy(container_name, "src", container_name, blob);
            promise.set_value();
        }},
    {"List", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string prefix, std::promise<void> promise)
        {
            attrib_cache_wrapper->list_blobs_segmented(container_name, "/", "marker", prefix, 10);
            promise.set_value();
        }},
};

// Helpers that look at the operation name.  Helps determine what the expected behavior is (whether or not to expect the calls to be serialized.)
bool includes_download_operation(std::string op1, std::string op2)
{
    return (op1.find("Download") == 0) || (op2.find("Download") == 0);
}

bool is_list_operation(std::string op)
{
    return op.find("List") == 0;
}

// Based on the operation name and the scenario, this calculates whether or not the test should expect the operations to be synchronized.
bool expect_synchronization(std::string first_operation, std::string second_operation, int scenario)
{
    if (includes_download_operation(first_operation, second_operation))
    {
        return false;
    }
    switch (scenario)
    {
        case 0:
            return true; // Operations on the same blob should synchronize.
            break;
        case 1:
            return false; // Different directories should never synchronize
            break;
        case 2:
            return is_list_operation(first_operation) || is_list_operation(second_operation); // Different blobs in the same directory should only synchronize if one or both operations are listing operations.
            break;
        default:
            std::cout << "No such scenario " << scenario;
            return false;
            break;
    }
}

TEST_P(AttribCacheSynchronizationTest, Run)
{
    std::string firstOperation = std::get<0>(GetParam());
    std::string secondOperation = std::get<1>(GetParam());
    int scenario = std::get<2>(GetParam());

    std::shared_ptr<std::mutex> m = std::make_shared<std::mutex>();
    std::shared_ptr<std::condition_variable> cv = std::make_shared<std::condition_variable>();
    std::shared_ptr<int> calls = std::make_shared<int>(0);
    std::shared_ptr<bool> sleep_finished = std::make_shared<bool>(false);

    // Note that listing operations here are a special case - instead of passing in the blob name, we need to pass in the prefix of the blob (meaning, the directory name)
    std::string input1 = is_list_operation(firstOperation) ? "dir1" : "dir1/bloba";

    std::string input2;
    //TODO: use an enum instead of an int.
    switch (scenario)
    {
        case 0:
            input2 = is_list_operation(secondOperation) ? "dir1" : "dir1/bloba"; // Same blob
            break;
        case 1:
            input2 = is_list_operation(secondOperation) ? "dir2" : "dir2/bloba"; // Different directory
            break;
        case 2:
            input2 = is_list_operation(secondOperation) ? "dir1" : "dir1/blobb"; // Same directory, different blob.
            break;
        default:
            FAIL() << "No such scenario " << scenario;
            break;
    }

    prep_mock(m, cv, calls, sleep_finished);

    std::promise<void> first_promise;
    std::future<void> first_future = first_promise.get_future();

    std::thread slow_call(fnMap[firstOperation], attrib_cache_wrapper, container_name, input1, std::move(first_promise));
    {
        // Ensure that the call in the new thread has started - without this, it's possible that the below call 
        // could happen prior to the get_properties call in a new thread.
        std::unique_lock<std::mutex> lk(*m);
        (*cv).wait(lk, [&] {return (*calls) > 0;});
    }

    EXPECT_FALSE(*sleep_finished);

    std::promise<void> unused;
    fnMap[secondOperation](attrib_cache_wrapper, container_name, input2, std::move(unused));

    if (expect_synchronization(firstOperation, secondOperation, scenario))
    {
        EXPECT_TRUE(*sleep_finished);
    }
    else
    {
        EXPECT_FALSE(*sleep_finished);
    }

    slow_call.join();
}

// Generates the list of operations
std::vector<std::string> getKeys()
{
    std::vector<std::string> keys;
    for (auto it = fnMap.begin(); it != fnMap.end(); it++)
    {
        keys.push_back(it->first);
    }
    return keys;
}

// Helper to generate a (more) friendly name for a given test, based on the test parameters.
std::string getTestName(::testing::TestParamInfo<std::tuple<std::string, std::string, int>> info)
{
    std::string scenario;
    switch (std::get<2>(info.param))
    {
        case 0:
            scenario = "SameBlob";
            break;
        case 1:
            scenario = "DiffDirectory";
            break;
        case 2:
            scenario = "DiffBlob";
            break;
        default:
            break;
    }    
    std::string ret;
    return ret + std::get<0>(info.param) + "Then" + std::get<1>(info.param) + scenario;
}

INSTANTIATE_TEST_CASE_P(AttribCacheTests, AttribCacheSynchronizationTest, ::testing::Combine(::testing::ValuesIn(getKeys()), ::testing::ValuesIn(getKeys()), ::testing::Values(0, 1, 2)), getTestName);