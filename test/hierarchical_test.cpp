#include <iostream>
#include <uuid/uuid.h>
#include "blobfuse.h"

using namespace std;

static std::shared_ptr<blob_client_wrapper> test_blob_client_wrapper;
std::string container_name;
std::string tmp_dir;

bool init_test()
{
	int ret = read_config("../connection.cfg");
    if(ret != 0)
	{
		cout << "Could not read config file";
		return false;
	}
    std::string blob_endpoint;
    std::string sas_token;
	test_blob_client_wrapper = std::make_shared<blob_client_wrapper>(blob_client_wrapper::blob_client_wrapper_init(config_options.accountName, config_options.accountKey, sas_token, 20, config_options.useHttps, blob_endpoint));
	return true;
}

// This runs before each test.  We create a container with a unique name, and create a test directory to be a sandbox.
bool SetUp()
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
    if(errno != 0)
	{
		cout << "SetUp - CreateContainer failed with errno = " << errno;
		return false;
	}

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

void write_to_file(std::string path, std::string text)
{
    std::ofstream output_stream(path, std::ios::binary);
    output_stream << text;
}

bool tests_list_blob_hierarchical()
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
    std::vector<list_blobs_segmented_item> blob_list_results = list_all_blobs(container_name, "/", "");
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

int main()
{
	tests_list_blob_hierarchical();
	return 0;
}