#include <uuid/uuid.h>
//#include "gtest/gtest.h"
#include "gmock/gmock.h"
#include "blobfuse.h"

using::testing::_;
using ::testing::Return;

// Used for GoogleMock
class MockBlobClient : public blob_client_wrapper {
public:
    MOCK_CONST_METHOD0(is_valid, bool());
    MOCK_METHOD5(list_blobs_segmented, list_blobs_segmented_response(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix, int maxresults));
    MOCK_METHOD4(put_blob, void(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata));
    MOCK_METHOD4(upload_block_blob_from_stream, void(const std::string &container, const std::string blob, std::istream &is, const std::vector<std::pair<std::string, std::string>> &metadata));
    MOCK_METHOD5(upload_file_to_blob, void(const std::string &sourcePath, const std::string &container, const std::string blob, const std::vector<std::pair<std::string, std::string>> &metadata, size_t parallel));
    MOCK_METHOD5(download_blob_to_stream, void(const std::string &container, const std::string &blob, unsigned long long offset, unsigned long long size, std::ostream &os));
    MOCK_METHOD5(download_blob_to_file, void(const std::string &container, const std::string &blob, const std::string &destPath, time_t &returned_last_modified, size_t parallel));
    MOCK_METHOD2(get_blob_property, blob_property(const std::string &container, const std::string &blob));
    MOCK_METHOD2(blob_exists, bool(const std::string &container, const std::string &blob));
    MOCK_METHOD2(delete_blob, void(const std::string &container, const std::string &blob));
    MOCK_METHOD4(start_copy, void(const std::string &sourceContainer, const std::string &sourceBlob, const std::string &destContainer, const std::string &destBlob));
};

// These tests primarily test correctness of the attr cache - both that the data is correct, and that data is being correctly cached.
// This file does not test synchronization behavior - that's in a different file.
// 
// Overall reminder regarding the GoogleTest assertion macros - 
// The EXPECT_* macros are used to validate correctness non-fatally.  Meaning, if an expectation fails, the test will fail, but will continue to run to completion.
// The ASSERT_* macros are supposed to be fatal.  If the assertion fails, the test fails, and the method returns at that point.  (Note that the caller will continue to run.)
class AttribCacheTest : public ::testing::Test {
public:
    // Usually using a strict mock is bad practice, but we're using it here because we're testing caching behavior.
    // Nice mocks or naggy mocks could ignore calls that should fail tests (because the cache is being used incorrectly.)
    std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient;
    std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper;
    std::string container_name;

    // This runs before each test.
    virtual void SetUp()
    {
       container_name = "container";
       mockClient = std::make_shared<::testing::StrictMock<MockBlobClient>>();
       attrib_cache_wrapper = std::make_shared<blob_client_attr_cache_wrapper>(mockClient);
    }

    virtual void TearDown()
    {
    }
};

// Helper methods for checking equality of blob properties and metadata
void assert_metadata_equal(std::vector<std::pair<std::string, std::string>>& left, std::vector<std::pair<std::string, std::string>>& right)
{
    ASSERT_EQ(left.size(), right.size()) << "blob_property objects not equal; differing metadata count.";
    std::vector<std::pair<std::string, std::string>> left_copy(left);
    std::vector<std::pair<std::string, std::string>> right_copy(right);
    std::sort(left_copy.begin(), left_copy.end());
    std::sort(right_copy.begin(), right_copy.end());
    auto mismatch = std::mismatch(left_copy.begin(), left_copy.end(), right_copy.begin());
    EXPECT_EQ(left_copy.end(), mismatch.first) << "Metadata not equal at left element = \"" << (*mismatch.first).first << "\"=\"" << (*mismatch.first).second << "\" and right element \"" << (*mismatch.second).first << "\"=\"" << (*mismatch.second).second << "\".";
}

void assert_blob_property_objects_equal(blob_property& left, blob_property& right)
{
    if (!left.valid() && !right.valid()) return; // two invalid objects are equal; aborting comparison.

    ASSERT_TRUE(left.valid()) << "blob_property objects not equal; left is invalid.";
    ASSERT_TRUE(right.valid()) << "blob_property objects not equal; right is invalid.";

    EXPECT_EQ(left.cache_control, right.cache_control) << "blob_property objects not equal; cache_control";
    EXPECT_EQ(left.content_disposition, right.content_disposition) << "blob_property objects not equal; content_disposition";
    EXPECT_EQ(left.content_encoding, right.content_encoding) << "blob_property objects not equal; content_encoding";
    EXPECT_EQ(left.content_language, right.content_language) << "blob_property objects not equal; content_language";
    EXPECT_EQ(left.size, right.size) << "blob_property objects not equal; size";
    EXPECT_EQ(left.content_md5, right.content_md5) << "blob_property objects not equal; content_md5";
    EXPECT_EQ(left.content_type, right.content_type) << "blob_property objects not equal; content_type";
    EXPECT_EQ(left.etag, right.etag) << "blob_property objects not equal; etag";
    EXPECT_EQ(left.copy_status, right.copy_status) << "blob_property objects not equal; copy_status";
    EXPECT_EQ(left.last_modified, right.last_modified) << "blob_property objects not equal; last_modified";
    // Add when implemented:
    // blob_type m_type;
    // azure::storage::lease_status m_lease_status;
    // azure::storage::lease_state m_lease_state;
    // azure::storage::lease_duration m_lease_duration;

    assert_metadata_equal(left.metadata, right.metadata);
}

void assert_list_item_equal(list_blobs_segmented_item &left, list_blobs_segmented_item &right)
{
    EXPECT_EQ(left.name, right.name);
    EXPECT_EQ(left.snapshot, right.snapshot);
    EXPECT_EQ(left.last_modified, right.last_modified);
    EXPECT_EQ(left.etag, right.etag);
    EXPECT_EQ(left.content_length, right.content_length);
    EXPECT_EQ(left.content_encoding, right.content_encoding);
    EXPECT_EQ(left.content_type, right.content_type);
    EXPECT_EQ(left.content_md5, right.content_md5);
    EXPECT_EQ(left.content_language, right.content_language);
    EXPECT_EQ(left.cache_control, right.cache_control);
    EXPECT_EQ(left.status, right.status);
    EXPECT_EQ(left.state, right.state);
    EXPECT_EQ(left.duration, right.duration);
    //EXPECT_EQ(left.copy_status, right.copy_status);
    EXPECT_EQ(left.is_directory, right.is_directory);
    assert_metadata_equal(left.metadata, right.metadata);
}

void assert_list_response_objects_equal(list_blobs_segmented_response &left, list_blobs_segmented_response& right)
{
    EXPECT_EQ(left.next_marker, right.next_marker);
    EXPECT_EQ(left.ms_request_id, right.ms_request_id);

    ASSERT_EQ(left.blobs.size(), right.blobs.size());
    for (size_t i = 0; i < left.blobs.size(); i++)
    {
        assert_list_item_equal(left.blobs[i], right.blobs[i]);
    }
}

// Helper for creating a sample blob_property object with some sample data.
blob_property create_blob_property(std::string etag, unsigned long long size)
{
    blob_property props(true);
    props.etag = etag;
    props.size = size;

    props.cache_control = "cache_control";
//    props.content_disposition = "content_disposition";  // Add when implemented
    props.content_encoding = "content_encoding";
    props.content_language = "content_language";
    props.content_md5 = "content_md5";
    props.content_type = "content_type";
    props.copy_status = "";


    props.last_modified = time(NULL);

    // Just some sample metadata
    props.metadata = {std::make_pair("k5", "v5"), std::make_pair("k1", "v1"), std::make_pair("k2", "v2"), std::make_pair("k3", "v3")};
    return props;
}

// Helper for converting a blob_property object into a list_blobs_segmented_item.
// TODO: Remove this once cpplite unifies these two types.
list_blobs_segmented_item blob_property_to_item(std::string name, blob_property prop, bool is_directory)
{
    list_blobs_segmented_item item;
    item.name = name;
    item.is_directory = is_directory;
    item.cache_control = prop.cache_control;
    item.content_encoding = prop.content_encoding;
    item.content_language = prop.content_language;
    item.content_length = prop.size;
    item.content_md5 = prop.content_md5;
    item.content_type = prop.content_type;
    item.etag = prop.etag;
    item.metadata = prop.metadata;
    //item.copy_status = prop.copy_status;

    char buf[30];
    std::time_t t = prop.last_modified;
    std::tm *pm;
    pm = std::gmtime(&t);
    size_t s = std::strftime(buf, 30, constants::date_format_rfc_1123, pm);
    item.last_modified = std::string(buf, s);

//  Add when implemented:
//    item.content_disposition = prop.content_disposition;
//    Lease status / state / duration

    return item;
}

// Base case - check that GetBlobProperties calls are cached.
TEST_F(AttribCacheTest, GetBlobPropertiesSingle)
{
    std::string blob = "blob";

    //TODO: replace with create_blob_property
    blob_property prop = create_blob_property("samepleEtag", 4);

    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob))
    .Times(1)
    .WillOnce(Return(prop));
    blob_property newprop = attrib_cache_wrapper->get_blob_property(container_name, blob);
    blob_property newprop2 = attrib_cache_wrapper->get_blob_property(container_name, blob);

    assert_blob_property_objects_equal(newprop, newprop2);
}

// Tests that regardless of multiple calls to get_property or ordering, each blob makes only one service call.
TEST_F(AttribCacheTest, GetBlobPropertiesMultiple)
{
    std::string blob1 = "blob1";
    std::string blob2 = "blob2";
    std::string blob3 = "blob3";

    blob_property prop1 = create_blob_property("etag1", 4);
    blob_property prop2 = create_blob_property("etag2", 15);
    blob_property prop3 = create_blob_property("etag3", 239817328401234ull); // larger than will fit in an int

    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob1))
    .Times(1)
    .WillOnce(Return(prop1));
    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob2))
    .Times(1)
    .WillOnce(Return(prop2));
    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob3))
    .Times(1)
    .WillOnce(Return(prop3));

    blob_property prop2copy1 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property prop1copy1 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property prop1copy2 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property prop2copy2 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property prop2copy3 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property prop1copy3 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property prop3copy1 = attrib_cache_wrapper->get_blob_property(container_name, blob3);
    blob_property prop1copy4 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property prop2copy4 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property prop1copy5 = attrib_cache_wrapper->get_blob_property(container_name, blob1);

    assert_blob_property_objects_equal(prop1, prop1copy1);
    assert_blob_property_objects_equal(prop1, prop1copy2);
    assert_blob_property_objects_equal(prop1, prop1copy3);
    assert_blob_property_objects_equal(prop1, prop1copy4);
    assert_blob_property_objects_equal(prop1, prop1copy5);
    assert_blob_property_objects_equal(prop2, prop2copy1);
    assert_blob_property_objects_equal(prop2, prop2copy2);
    assert_blob_property_objects_equal(prop2, prop2copy3);
    assert_blob_property_objects_equal(prop2, prop2copy4);
    assert_blob_property_objects_equal(prop3, prop3copy1);
}

// Check that listing operations cache the returned blob properties
TEST_F(AttribCacheTest, GetBlobPropertiesListSimple)
{
    std::string blob1 = "blob1";
    std::string blob2 = "blob2";
    std::string blob3 = "blob3";

    blob_property prop1 = create_blob_property("etag1", 4);
    blob_property prop2 = create_blob_property("etag2", 15);
    blob_property prop3 = create_blob_property("etag3", 239817328401234ull); // larger than will fit in an int

    list_blobs_segmented_response list_response;
    list_response.next_marker = "marker";
    list_response.blobs.push_back(blob_property_to_item(blob1, prop1, false));
    list_response.blobs.push_back(blob_property_to_item(blob2, prop2, true)); // Ensure that directories don't get cached
    list_response.blobs.push_back(blob_property_to_item(blob3, prop3, false));

    EXPECT_CALL(*mockClient, list_blobs_segmented(container_name, "/", "token", "prefix", 10000))
    .Times(1)
    .WillOnce(Return(list_response));
    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob2))
    .Times(1)
    .WillOnce(Return(prop2));

    list_blobs_segmented_response list_response_cache = attrib_cache_wrapper->list_blobs_segmented(container_name, "/", "token", "prefix", 10000);
    blob_property prop1_1 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property prop2_1 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property prop3_1 = attrib_cache_wrapper->get_blob_property(container_name, blob3);

    assert_list_response_objects_equal(list_response, list_response_cache);

    assert_blob_property_objects_equal(prop1, prop1_1);
    assert_blob_property_objects_equal(prop2, prop2_1);
    assert_blob_property_objects_equal(prop3, prop3_1);
}

TEST_F(AttribCacheTest, GetBlobPropertiesListRepeated)
{
    // Here we will test the interaction of multiple get_blob_property and list_blobs calls.
    // We'll make two list_blobs calls, with get_blob_property calls before, between, and after.
    // Different blobs will return different values at the various calls, to test different behavior.
    // 
    // blob1 is the base case, it's not modified between calls.
    // blob2 will change between calls, ensuring that the cache is updated properly.
    // blob3 will not be included in the listing results, ensuring that the data in the cache is still valid even in this case.
    // blob4 will be invalidated between calls, to make sure that the cache data is re-applied in a list call in this case.
    // blob5 will be invalidated between calls, to make sure that the cache data is re-applied in a get_properties call in this case.

    std::string blob1 = "blob1";
    std::string blob2 = "blob2";
    std::string blob3 = "blob3";
    std::string blob4 = "blob4";
    std::string blob5 = "blob5";

    blob_property prop1 = create_blob_property("etag1", 4);
    blob_property prop2_v0 = create_blob_property("etag2_0", 29);
    blob_property prop2_v1 = create_blob_property("etag2_1", 15);
    blob_property prop2_v2 = create_blob_property("etag2_2", 43);
    blob_property prop3 = create_blob_property("etag3", 239817328401234ull); // larger than will fit in an int
    blob_property prop4_v1 = create_blob_property("etag4_1", 0);
    blob_property prop4_v2 = create_blob_property("etag4_2", 1);
    blob_property prop5_v1 = create_blob_property("etag5_1", 2);
    blob_property prop5_v2 = create_blob_property("etag5_2", 3);

    list_blobs_segmented_response list_response_1;
    list_response_1.next_marker = "marker";
    list_response_1.blobs.push_back(blob_property_to_item(blob1, prop1, false));
    list_response_1.blobs.push_back(blob_property_to_item(blob2, prop2_v1, false));
    list_response_1.blobs.push_back(blob_property_to_item(blob4, prop4_v1, false));
    list_response_1.blobs.push_back(blob_property_to_item(blob5, prop5_v1, false));


    list_blobs_segmented_response list_response_2;
    list_response_2.next_marker = "marker";
    list_response_2.blobs.push_back(blob_property_to_item(blob1, prop1, false));
    list_response_2.blobs.push_back(blob_property_to_item(blob2, prop2_v2, false));
    list_response_2.blobs.push_back(blob_property_to_item(blob4, prop4_v2, false));
    list_response_2.blobs.push_back(blob_property_to_item(blob5, prop5_v2, false));


    {
        ::testing::InSequence seq; // Expectations defined until this goes out-of-scope are validated in order.

        EXPECT_CALL(*mockClient, get_blob_property(container_name, blob2))
        .Times(1)
        .WillOnce(Return(prop2_v0));
        EXPECT_CALL(*mockClient, list_blobs_segmented(container_name, "/", "token", "prefix", 10000))
        .Times(1)
        .WillOnce(Return(list_response_1));
        EXPECT_CALL(*mockClient, get_blob_property(container_name, blob3))
        .Times(1)
        .WillOnce(Return(prop3));
        EXPECT_CALL(*mockClient, delete_blob(container_name, blob4))
        .Times(1);
        EXPECT_CALL(*mockClient, delete_blob(container_name, blob5))
        .Times(1);        
        EXPECT_CALL(*mockClient, get_blob_property(container_name, blob5))
        .Times(1)
        .WillOnce(Return(prop5_v2));
        EXPECT_CALL(*mockClient, list_blobs_segmented(container_name, "/", "token", "prefix", 10000))
        .Times(1)
        .WillOnce(Return(list_response_2));
    }

    blob_property propcache2_0 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    list_blobs_segmented_response list_response_cache_1 = attrib_cache_wrapper->list_blobs_segmented(container_name, "/", "token", "prefix", 10000);
    blob_property propcache1_1 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property propcache2_1 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property propcache3_1 = attrib_cache_wrapper->get_blob_property(container_name, blob3);
    blob_property propcache4_1 = attrib_cache_wrapper->get_blob_property(container_name, blob4);
    blob_property propcache5_1 = attrib_cache_wrapper->get_blob_property(container_name, blob5);

    attrib_cache_wrapper->delete_blob(container_name, blob4); // Invalidate blobs 4 and 5
    attrib_cache_wrapper->delete_blob(container_name, blob5);

    blob_property propcache5_post_invalidate = attrib_cache_wrapper->get_blob_property(container_name, blob5);

    list_blobs_segmented_response list_response_cache_2 = attrib_cache_wrapper->list_blobs_segmented(container_name, "/", "token", "prefix", 10000);
    blob_property propcache1_2 = attrib_cache_wrapper->get_blob_property(container_name, blob1);
    blob_property propcache2_2 = attrib_cache_wrapper->get_blob_property(container_name, blob2);
    blob_property propcache3_2 = attrib_cache_wrapper->get_blob_property(container_name, blob3);
    blob_property propcache4_2 = attrib_cache_wrapper->get_blob_property(container_name, blob4);
    blob_property propcache5_2 = attrib_cache_wrapper->get_blob_property(container_name, blob5);

    assert_list_response_objects_equal(list_response_1, list_response_cache_1);
    assert_list_response_objects_equal(list_response_2, list_response_cache_2);

    assert_blob_property_objects_equal(prop2_v0, propcache2_0);

    assert_blob_property_objects_equal(prop1, propcache1_1);
    assert_blob_property_objects_equal(prop2_v1, propcache2_1);
    assert_blob_property_objects_equal(prop3, propcache3_1);
    assert_blob_property_objects_equal(prop4_v1, propcache4_1);
    assert_blob_property_objects_equal(prop5_v1, propcache5_1);

    assert_blob_property_objects_equal(prop5_v2, propcache5_post_invalidate);

    assert_blob_property_objects_equal(prop1, propcache1_2);
    assert_blob_property_objects_equal(prop2_v2, propcache2_2);
    assert_blob_property_objects_equal(prop3, propcache3_2);
    assert_blob_property_objects_equal(prop4_v2, propcache4_2);
    assert_blob_property_objects_equal(prop5_v2, propcache5_2);
}

// These tests ensure that methods other than get_blob_properties and list_blobs invalidate the cache when called.
// We use GoogleTest's parameterized testing to generate one test per method.
class AttribCacheInvalidateCacheTest : public AttribCacheTest, public ::testing::WithParamInterface<std::string> {
};

// Maps the name of an operation to the code needed to call the operation under test.
std::map<std::string, std::function<void(std::shared_ptr<blob_client_attr_cache_wrapper>, std::string, std::string)>> operationMap = 
{
    {"Put", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->put_blob("source_path", container_name, blob, metadata);
        }},
    {"UploadFromStream", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            std::stringstream is;
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->upload_block_blob_from_stream(container_name, blob, is, metadata);
        }},
    {"UploadFromFile", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            std::vector<std::pair<std::string, std::string>> metadata;
            attrib_cache_wrapper->upload_file_to_blob("source_path", container_name, blob, metadata, 10);
        }},
    {"DownloadToStream", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            std::stringstream os;
            attrib_cache_wrapper->download_blob_to_stream(container_name, blob, 0, 10, os);
        }},
    {"DownloadToFile", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            time_t lmt;
            attrib_cache_wrapper->download_blob_to_file(container_name, blob, "dest_path", lmt, 10);
        }},
    {"Exists", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            attrib_cache_wrapper->blob_exists(container_name, blob);
        }},
    {"Delete", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            attrib_cache_wrapper->delete_blob(container_name, blob);
        }},
    {"Copy", [](std::shared_ptr<blob_client_attr_cache_wrapper> attrib_cache_wrapper, std::string container_name, std::string blob)
        {
            attrib_cache_wrapper->start_copy(container_name, "src", container_name, blob);
        }}, 
};

// Maps the name of an operation to the code needed to set up the expectation for that operation on the mock.
// Needed because we are using a StrictMock, and we want to validate the exact call sequence.
std::map<std::string, std::function<void(std::shared_ptr<::testing::StrictMock<MockBlobClient>>, std::string, std::string, ::testing::Sequence)>> expectationMap = 
{
    {"Put", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, put_blob(_, container_name, blob_name, _))
        .Times(1)
        .InSequence(seq);
    }},
    {"UploadFromStream", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, upload_block_blob_from_stream(container_name, blob_name, _, _))
        .Times(1)
        .InSequence(seq);
    }},
    {"UploadFromFile", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, upload_file_to_blob(_, container_name, blob_name, _, _))
        .Times(1)
        .InSequence(seq);
    }},
    {"DownloadToStream", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, download_blob_to_stream(container_name, blob_name, _, _, _))
        .Times(1)
        .InSequence(seq);
    }},
    {"DownloadToFile", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, download_blob_to_file(container_name, blob_name, _, _, _))
        .Times(1)
        .InSequence(seq);
    }},
    {"Exists", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>>, std::string, std::string, ::testing::Sequence)
    {
        // Exists is a special case - it calls this.get_blob_property internally, which (in this case) should be cached, so we don't expect anything.
    }}, 
    {"Delete", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, delete_blob(container_name, blob_name))
        .Times(1)
        .InSequence(seq);
    }},
    {"Copy", [](std::shared_ptr<::testing::StrictMock<MockBlobClient>> mockClient, std::string container_name, std::string blob_name, ::testing::Sequence seq)
    {
        EXPECT_CALL(*mockClient, start_copy(container_name, _, container_name, blob_name))
        .Times(1)
        .InSequence(seq);
    }},
};

// For each operation, whether or not the test should expect the operation to invalidate the cache.
std::map<std::string, bool> expectInvalidate = 
{
    {"Put", true},
    {"UploadFromStream", true},
    {"UploadFromFile", true},
    {"DownloadToStream", false},
    {"DownloadToFile", false},
    {"Exists", false},
    {"Delete", true},
    {"Copy", true},
};


TEST_P(AttribCacheInvalidateCacheTest, Run)
{
    std::string operation_name = GetParam();
    std::string blob_name = "blob";

    blob_property prop1 = create_blob_property("etag1", 4);
    blob_property prop2 = create_blob_property("etag2", 17);

    // This is a way to "checkpoint" calls - used with a Sequence, it helps ensure that the expectations are being called from the correct point.
    // Otherwise, it might be unclear exactly which get_blob_property call was being matched in the mock.
    ::testing::MockFunction<void(std::string check_point_name)> check;
    ::testing::Sequence seq;

    EXPECT_CALL(*mockClient, get_blob_property(container_name, blob_name)).Times(1).InSequence(seq).WillOnce(Return(prop1));
    EXPECT_CALL(check, Call("1")).Times(1).InSequence(seq);
    EXPECT_CALL(check, Call("2")).Times(1).InSequence(seq);
    expectationMap[operation_name](mockClient, container_name, blob_name, seq);
    EXPECT_CALL(check, Call("3")).Times(1).InSequence(seq);
    if (expectInvalidate[operation_name])
    {
        EXPECT_CALL(*mockClient, get_blob_property(container_name, blob_name)).Times(1).InSequence(seq)
        .WillOnce(Return(prop2));
    }

    blob_property prop_cache1 = attrib_cache_wrapper->get_blob_property(container_name, blob_name);
    check.Call("1");
    blob_property prop_cache2 = attrib_cache_wrapper->get_blob_property(container_name, blob_name);
    check.Call("2");
    operationMap[operation_name](attrib_cache_wrapper, container_name, blob_name);
    blob_property prop_cache3 = attrib_cache_wrapper->get_blob_property(container_name, blob_name);
    check.Call("3");

    assert_blob_property_objects_equal(prop1, prop_cache1);
    assert_blob_property_objects_equal(prop1, prop_cache2);
    assert_blob_property_objects_equal(expectInvalidate[operation_name] ? prop2 : prop1, prop_cache3);
}

// Helpers for instantiating the parameterized tests
std::vector<std::string> getKeys2()
{
    std::vector<std::string> keys;
    for (auto it = operationMap.begin(); it != operationMap.end(); it++)
    {
        keys.push_back(it->first);
    }
    return keys;
}

// Helper to generate a more-informative test name
std::string getTestName(::testing::TestParamInfo<std::string> info)
{
    return info.param;
}

INSTANTIATE_TEST_CASE_P(AttribCacheTests, AttribCacheInvalidateCacheTest, ::testing::ValuesIn(getKeys2()), getTestName);

// TODO: move main() into a separate file; it should exist only once for the 'blobfusetests' application.
int main(int argc, char** argv) {
  ::testing::InitGoogleMock(&argc, argv);
  return RUN_ALL_TESTS();
}