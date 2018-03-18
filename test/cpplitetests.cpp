#include <uuid/uuid.h>
#include <ftw.h>

#include "gtest/gtest.h"
//#include "gmock/gmock.h"
#include "blobfuse.h"

#define CHECK_STRINGS(LEFTSTRING, RIGHTSTRING) ASSERT_EQ(0, LEFTSTRING.compare(RIGHTSTRING)) << "Strings failed equality comparison.  " << #LEFTSTRING << " is " << LEFTSTRING << ", " << #RIGHTSTRING << " is " << RIGHTSTRING << ".  "

TEST(Blobfuse, MapErrno)
{
    EXPECT_EQ(ENOENT, map_errno(404)) << "HTTP error 404 should map to errno ENOENT (which is " << ENOENT << ").  Actual = " << map_errno(404);
}


int rm_helper(const char *fpath, const struct stat * /*sb*/, int tflag, struct FTW * /*ftwbuf*/)
{
    if (tflag == FTW_DP)
    {
        errno = 0;
        int ret = rmdir(fpath);
        return ret;
    }
    else
    {
        errno = 0;
        int ret = unlink(fpath);
        return ret;
    }
}

// Delete the entire contents of tmpPath.
void destroy_path(std::string path_to_destroy)
{
    errno = 0;
    // FTW_DEPTH instructs FTW to do a post-order traversal (children of a directory before the actual directory.)
    nftw(path_to_destroy.c_str(), rm_helper, 20, FTW_DEPTH); 
}

class BlobClientWrapperTest : public ::testing::Test {
public:
    static std::shared_ptr<blob_client_wrapper> test_blob_client_wrapper;

    // This runs once, before any tests in "BlobClientWrapperTest"
    static void SetUpTestCase() {
        int ret = read_config("../connection.cfg");
        ASSERT_EQ(0, ret) << "Read config failed.";
        std::string blob_endpoint;
        std::string sas_token;
        test_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(str_options.accountName, str_options.accountKey, sas_token, 20, str_options.use_https, blob_endpoint));
    }

    std::string container_name;
    std::string tmp_dir;

    // This runs before each test.  We create a container with a unique name, and create a test directory to be a sandbox.
    virtual void SetUp()
    {
        uuid_t container_uuid;
        uuid_generate( (unsigned char *)&container_uuid );

        char container_name_uuid[37];
        uuid_unparse_lower(container_uuid, container_name_uuid);

        std::string container_name_prefix = "container";
        container_name = container_name_prefix + container_name_uuid;
        container_name.erase(std::remove(container_name.begin(), container_name.end(), '-'), container_name.end());
        errno = 0;
        test_blob_client_wrapper->create_container(container_name);
        ASSERT_EQ(0, errno) << "SetUp - CreateContainer failed with errno = " << errno;

        tmp_dir = "/tmp/blobfuseteststmp";
        errno = 0;
        struct stat buf;
        int statret = stat(tmp_dir.c_str(), &buf);
        if (statret == 0)
        {
            errno = 0;
            destroy_path(tmp_dir);
            ASSERT_EQ(0, errno) << "SetUp - cleanup of old tmp directory failed with errno " << errno;
        }
        errno = 0;
        mkdir(tmp_dir.c_str(), 0777);
        ASSERT_EQ(0, errno) << "SetUp - tmp dir creation failed with errno " << errno;
    }

    virtual void TearDown()
    {
        test_blob_client_wrapper->delete_container(container_name);
        destroy_path(tmp_dir);
    }
    void run_upload_download(size_t file_size);
};

std::shared_ptr<blob_client_wrapper> BlobClientWrapperTest::test_blob_client_wrapper = NULL;

// Helper method to follow continuation tokens and list all blobs.  C&P from blobfuse code, we should probably move it to cpplite, although not until we figure out proper retries.
std::vector<list_blobs_hierarchical_item> list_all_blobs(std::string container, std::string delimiter, std::string prefix)
{
    std::vector<list_blobs_hierarchical_item> results;

    std::string continuation;

    std::string prior;
    do
    {
        errno = 0;
        list_blobs_hierarchical_response response = BlobClientWrapperTest::test_blob_client_wrapper->list_blobs_hierarchical(container, delimiter, continuation, prefix);
        if (errno == 0)
        {
            continuation = response.next_marker;
            if(response.blobs.size() > 0)
            {
                auto begin = response.blobs.begin();
                if(response.blobs[0].name == prior)
                {
                    std::advance(begin, 1);
                }
                results.insert(results.end(), begin, response.blobs.end());
                prior = response.blobs.back().name;
            }
        }
        else
        {
            return results; // errno should be set already.
        }
    } while (continuation.size() > 0);

    return results;
}



void write_to_file(std::string path, std::string text)
{
    std::ofstream output_stream(path, std::ios::binary);
    output_stream << text;
}

void read_from_file(std::string path, std::string &text)
{
    std::ifstream input_stream(path, std::ios::binary);
    std::stringstream string_stream;
    string_stream << input_stream.rdbuf();
    text =  string_stream.str();
}

// Test to ensure that we can round-trip a small blob, and see it in a listing operation.
TEST_F(BlobClientWrapperTest, BlobPutDownload)
{
    errno = 0;
    std::vector<list_blobs_hierarchical_item> blobs = list_all_blobs(container_name, "/", "");
    ASSERT_EQ(0, errno);
    ASSERT_EQ(0, blobs.size());

    // Create a file
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);

    std::string blob_1_name("blob1name");
    errno = 0;
    test_blob_client_wrapper->put_blob(file_path, container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "Put blob failed";

    errno = 0;
    blobs = list_all_blobs(container_name, "/", "");
    ASSERT_EQ(0, errno);
    ASSERT_EQ(1, blobs.size());

    std::string dest_path = tmp_dir + "/destfile";

    time_t lmt;
    test_blob_client_wrapper->download_blob_to_file(container_name, blob_1_name, dest_path, lmt);

    std::string result_text;
    read_from_file(dest_path, result_text);

    ASSERT_EQ(0, file_text.compare(result_text)) << "Strings not equal.  file_text = " << file_text << ", result_text = " << result_text;
}

// Helper method to create random file.
void write_random_data_to_file(std::string path, unsigned int seed, size_t count)
{
    std::ofstream file_stream(path, std::ios::binary);
    // Using a low-quality RNG here for speed, no need for high-quality randomness
    std::minstd_rand r(seed);

    for (size_t i = 0; i < count; i += 4 /* sizeof uint_fast32_t */)
    {
        uint_fast32_t val = r();
        file_stream.write(reinterpret_cast<char*>(&val), 4);
    }
}

// Helper method to validate the contents of a file match what is expected from the RNG, with a given seed.
void read_file_data_and_validate(std::string path, unsigned int seed, size_t count)
{
    std::ifstream file_stream(path, std::ios::ate | std::ios::binary);
    size_t file_size = file_stream.tellg();
    ASSERT_EQ(count, file_size) << "File size incorrect.  Expected = " << count << ", actual = " << file_size;
    file_stream.seekg(0);

    std::minstd_rand r(seed);
        for (size_t i = 0; i < count; i += 4 /* sizeof uint_fast32_t */)
    {
        uint_fast32_t expect_val = r();
        uint_fast32_t actual_val;
        file_stream.read(reinterpret_cast<char*>(&actual_val), 4);
        ASSERT_EQ(expect_val, actual_val) << "File data incorrect at position "<< i << ".  Expected = " << expect_val << ", actual = " << actual_val;
    }
}

// Helper method to validate uploading and download a single file of arbitrary size
void BlobClientWrapperTest::run_upload_download(size_t file_size)
{
    errno = 0;
    std::vector<list_blobs_hierarchical_item> blobs = list_all_blobs(container_name, "/", "");
    ASSERT_EQ(0, errno);
    ASSERT_EQ(0, blobs.size());

    // Create a file
    std::string file_path = tmp_dir + "/tmpfile";

    unsigned int seed = 17;
    write_random_data_to_file(file_path, seed, file_size);

    std::string blob_1_name("blob1name");
    test_blob_client_wrapper->upload_file_to_blob(file_path, container_name, blob_1_name);

    errno = 0;
    blobs = list_all_blobs(container_name, "/", "");
    ASSERT_EQ(0, errno);
    ASSERT_EQ(1, blobs.size());
    ASSERT_EQ(file_size, blobs[0].content_length) << "Blob found, but size incorrect.";

    std::string dest_path = tmp_dir + "/destfile";

    time_t lmt;
    test_blob_client_wrapper->download_blob_to_file(container_name, blob_1_name, dest_path, lmt);
    // TODO: test that download returns correct blob properties once implemented.

    read_file_data_and_validate(dest_path, seed, file_size);
}

TEST_F(BlobClientWrapperTest, BlobUploadDownloadSmall)
{
    run_upload_download(5 * 1024);
}

TEST_F(BlobClientWrapperTest, BlobUploadDownloadMedium)
{
    run_upload_download(5 * 1024 * 1024);
}

TEST_F(BlobClientWrapperTest, BlobUploadDownloadLarge)
{
    run_upload_download(1 * 1024 * 1024 * 1024);  // Comment this test out if the test pass is taking too long during rapid iteration.
}


TEST_F(BlobClientWrapperTest, BlobUploadDownloadFailures)
{
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);
    std::string blob_1_name("blob1name");

    // Test that HTTP failures in put blob are correctly reported.
    errno = 0;
    test_blob_client_wrapper->put_blob(file_path, "notacontainer", blob_1_name);
//    ASSERT_EQ(404, errno) << "Put blob did not fail as expected - container doesn't exist.";//  TODO: Investigate why this doesn't fail as it should.

    // Test that file-system related failures in put blob are correctly reported.
    errno = 0;
    test_blob_client_wrapper->put_blob(file_path + "notafile", container_name, blob_1_name);
//    ASSERT_EQ(404, errno) << "Put blob did not fail as expected - input file doesn't exist.";//  TODO: Investigate why this doesn't fail as it should.

    // Test that HTTP failures in upload file are correctly reported.
    errno = 0;
    test_blob_client_wrapper->upload_file_to_blob(file_path, "notacontainer", blob_1_name);
//    ASSERT_EQ(404, errno) << "Upload file to blob did not fail as expected - container doesn't exist.";//  TODO: Investigate why this doesn't fail as it should.

    // Test that file-system related failures in upload file are correctly reported.
    errno = 0;
    test_blob_client_wrapper->upload_file_to_blob(file_path + "notafile", container_name, blob_1_name);
    ASSERT_EQ(ENOENT, errno) << "Upload file to blob did not fail as expected - input file doesn't exist.";

    // Test that HTTP failures in download blob are correctly reported.
    std::string dest_path = tmp_dir + "/destfile";
    errno = 0;
    time_t lmt;
    test_blob_client_wrapper->download_blob_to_file(container_name, "notablob", dest_path, lmt);
    ASSERT_EQ(404, errno) << "Download blob to file did not fail as expected - input file doesn't exist.";
}

TEST_F(BlobClientWrapperTest, GetBlobProperties)
{
    // Create a file
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);

    std::string blob_1_name("blob1name");

    // Test that HTTP failures are correctly reported
    errno = 0;
    blob_property props = test_blob_client_wrapper->get_blob_property(container_name, blob_1_name);
//    ASSERT_EQ(404, errno) << "Errno incorrect for get_blob_property";  TODO: investigate why this test is failing.

    time_t now = time(NULL);
    struct tm * gmtmp = gmtime(&now);
    time_t utcnow = mktime(gmtmp);
    errno = 0;
    test_blob_client_wrapper->put_blob(file_path, container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "put_blob failed with errno = " << errno;

    // Test that get_blob_property succeeds on a real blob, and reports the correct blob size & LMT.
    errno = 0;
    props = test_blob_client_wrapper->get_blob_property(container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "get_blob_property failed";
    ASSERT_EQ(file_text.size(), props.size) << "Incorrect blob size found.";
    ASSERT_TRUE(std::abs(std::difftime(props.last_modified, utcnow)) < 30) << "Time difference between expected and actual LMT from get_properties too large"; // Give some room for potential clock skew between local and service timestamp.

    std::string dest_path = tmp_dir + "/destfile";
    errno = 0;
    time_t lmt;
    test_blob_client_wrapper->download_blob_to_file(container_name, blob_1_name, dest_path, lmt);
    ASSERT_EQ(0, errno) << "download_blob_to_file failed";
    ASSERT_TRUE(std::difftime(lmt, props.last_modified) == 0) << "Incorrect timestamp from download_blob_to_file."; // Timestamps should be exactly equal
}
TEST_F(BlobClientWrapperTest, BlobExistsDelete)
{
    // Create a file
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);

    std::string blob_1_name("blob1name");

    // Test blob_exists - error case
    errno = 0;
    ASSERT_FALSE(test_blob_client_wrapper->blob_exists(container_name, blob_1_name)) << "Blob found when it should not be.";
    ASSERT_EQ(0, errno) << "Blob not existing causes errno to be non-0 for blob_exists.  Errno = " << errno;

    // Test blob_delete - error case (404)
    errno = 0;
    test_blob_client_wrapper->delete_blob(container_name, blob_1_name);
    ASSERT_EQ(404, errno) << "delete_blob errno non-0 for failure case";

    errno = 0;
    test_blob_client_wrapper->put_blob(file_path, container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "put_blob failed";

    // Test bllb_exists - success case
    errno = 0;
    ASSERT_TRUE(test_blob_client_wrapper->blob_exists(container_name, blob_1_name)) << "Blob not found";
    ASSERT_EQ(0, errno) << "blob_exists failed";

    // Test blob_delete - both that it reports success, and that the blob was actually deleted.
    errno = 0;
    test_blob_client_wrapper->delete_blob(container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "delete_blob failed";

    errno = 0;
    ASSERT_FALSE(test_blob_client_wrapper->blob_exists(container_name, blob_1_name)) << "Blob found when it should not be.";
    ASSERT_EQ(0, errno) << "Blob not existing causes errno to be non-0 for blob_exists.  Errno = " << errno;
}

// TODO: reduce duplicated code in this method
TEST_F(BlobClientWrapperTest, CopyBlob)
{
    // Setup

    // Create a file
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);

    std::string blob_1_name("blob1name");

    errno = 0;
    test_blob_client_wrapper->put_blob(file_path, container_name, blob_1_name);
    ASSERT_EQ(0, errno) << "put_blob failed";

    // Test copy blob over an existing blob
    std::string file_path_2 = tmp_dir + "/tmpfile2";
    std::string file_text_2 = "some different file text here.";

    write_to_file(file_path, file_text);
    std::string blob_2_name("blob2name");

    errno = 0;
    test_blob_client_wrapper->put_blob(file_path_2, container_name, blob_2_name);
    ASSERT_EQ(0, errno) << "put_blob failed";

    errno = 0;
    blob_property props = test_blob_client_wrapper->get_blob_property(container_name, blob_2_name);
    ASSERT_EQ(0, errno) << "get_blob_property failed";
//    ASSERT_EQ(file_text_2.size(), props.size);
    ASSERT_TRUE(props.copy_status.empty()) << "copy_status not empty as expected.";

    errno = 0;
    test_blob_client_wrapper->start_copy(container_name, blob_1_name, container_name, blob_2_name);
    ASSERT_EQ(0, errno) << "start_copy failed";

    do
    {
        errno = 0;
        props = test_blob_client_wrapper->get_blob_property(container_name, blob_2_name);
        ASSERT_EQ(0, errno) << "get_blob_property failed";
        sleep(1);
    } while (props.copy_status.compare("success\r\n") != 0); // HTTP spec specifies CRLF, and we aren't trimming (but probably should).
    ASSERT_EQ(file_text.size(), props.size);

    // Test copy blob to a new blob
    std::string blob_3_name("blob3name");

    errno = 0;
    test_blob_client_wrapper->start_copy(container_name, blob_1_name, container_name, blob_3_name);
    ASSERT_EQ(0, errno) << "start_copy failed";

    do
    {
        errno = 0;
        props = test_blob_client_wrapper->get_blob_property(container_name, blob_3_name);
        ASSERT_EQ(0, errno) << "get_blob_property failed";
        sleep(1);
    } while (props.copy_status.compare("success\r\n") != 0); // HTTP spec specifies CRLF, and we aren't trimming (but probably should).
    ASSERT_EQ(file_text.size(), props.size);

    // Failure case
    errno = 0;
    test_blob_client_wrapper->start_copy(container_name, blob_1_name, "notacontainer", blob_2_name);
    ASSERT_EQ(404, errno) << "start_copy did not as expected";

}

TEST_F(BlobClientWrapperTest, ListBlobsHierarchial)
{
    // Setup a series of blobs in a pretend file structure
    std::string file_path = tmp_dir + "/tmpfile";
    std::string file_text = "some file text here.";

    write_to_file(file_path, file_text);

    std::vector<std::string> blob_names;
    blob_names.push_back("bloba");
    blob_names.push_back("blobb");
    blob_names.push_back("blobc");
    blob_names.push_back("zblob");
    blob_names.push_back("dira/blobd");
    blob_names.push_back("dira/blobe");
    blob_names.push_back("dirb/blobf");
    blob_names.push_back("dirb/blobg");
    blob_names.push_back("dira/dirc/blobd");
    blob_names.push_back("dira/dirc/blobe");

    for (uint i = 0; i < blob_names.size(); i++)
    {
        errno = 0;
        test_blob_client_wrapper->put_blob(file_path, container_name, blob_names[i]);
        ASSERT_EQ(0, errno) << "put_blob failed for blob << " << blob_names[i];
    }

    // Validate that all blobs and blob "directories" are correctly found for given prefixes
    errno = 0;
    std::vector<list_blobs_hierarchical_item> blob_list_results = list_all_blobs(container_name, "/", "");
    ASSERT_EQ(0, errno) << "list_all_blobs failed for empty prefix";
    ASSERT_EQ(6, blob_list_results.size()) << "Incorrect number of blob entries found.";

    CHECK_STRINGS(blob_list_results[0].name, blob_names[0]);
    ASSERT_FALSE(blob_list_results[0].is_directory);
    CHECK_STRINGS(blob_list_results[1].name, blob_names[1]);
    ASSERT_FALSE(blob_list_results[1].is_directory);
    CHECK_STRINGS(blob_list_results[2].name, blob_names[2]);
    ASSERT_FALSE(blob_list_results[2].is_directory);
    CHECK_STRINGS(blob_list_results[3].name, blob_names[3]);
    ASSERT_FALSE(blob_list_results[3].is_directory);
    CHECK_STRINGS(blob_list_results[4].name, "dira/");
    ASSERT_TRUE(blob_list_results[4].is_directory);
    CHECK_STRINGS(blob_list_results[5].name, "dirb/");
    ASSERT_TRUE(blob_list_results[5].is_directory);

    errno = 0;
    blob_list_results = list_all_blobs(container_name, "/", "dira/");
    ASSERT_EQ(0, errno) << "list_all_blobs failed for prefix dira/";
    ASSERT_EQ(3, blob_list_results.size()) << "Incorrect number of blob entries found.";

    CHECK_STRINGS(blob_list_results[0].name, blob_names[4]);
    ASSERT_FALSE(blob_list_results[0].is_directory);
    CHECK_STRINGS(blob_list_results[1].name, blob_names[5]);
    ASSERT_FALSE(blob_list_results[1].is_directory);
    CHECK_STRINGS(blob_list_results[2].name, "dira/dirc/");
    ASSERT_TRUE(blob_list_results[2].is_directory);

    errno = 0;
    blob_list_results = list_all_blobs(container_name, "/", "dira/dirc/");
    ASSERT_EQ(0, errno) << "list_all_blobs failed for prefix dira/dirc/";
    ASSERT_EQ(2, blob_list_results.size()) << "Incorrect number of blob entries found.";

    CHECK_STRINGS(blob_list_results[0].name, blob_names[8]);
    ASSERT_FALSE(blob_list_results[0].is_directory);
    CHECK_STRINGS(blob_list_results[1].name, blob_names[9]);
    ASSERT_FALSE(blob_list_results[1].is_directory);

    errno = 0;
    list_all_blobs(container_name + 'x', "/", "");
    ASSERT_EQ(404, errno) << "Listing did not fail as expected.";

    errno = 0;
    blob_list_results = list_all_blobs(container_name, "/", "notaprefix");
    ASSERT_EQ(0, errno) << "Listing failed for zero results";
    ASSERT_EQ(0, blob_list_results.size()) << "Incorrect number of blobs found.";
}
