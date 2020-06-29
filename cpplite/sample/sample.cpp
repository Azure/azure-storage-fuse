#include "storage_credential.h"
#include "storage_account.h"
#include "blob/blob_client.h"

#include <iostream>
#include <fstream>
#include <sstream>

int main()
{
    using namespace azure::storage_lite;

    std::string account_name = "YOUR_STORAGE_ACCOUNT";
    std::string account_key = "";
    std::string container_name = "my-sample-container";
    std::string blob_name = "my-sample-blob";

    // Create a file for uploading later
    std::string sample_file = "sample-file";
    std::ofstream fout("sample-file");
    fout << "Hello world!\n";
    fout.close();

    std::shared_ptr<storage_credential> cred = std::make_shared<shared_key_credential>(account_name, account_key);
    std::shared_ptr<storage_account> account = std::make_shared<storage_account>(account_name, cred, /* use_https */ true);
    blob_client client(account, 16);

    auto ret = client.create_container(container_name).get();
    if (!ret.success())
    {
        std::cout << "Failed to create container, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }

    std::ifstream fin(sample_file, std::ios_base::in | std::ios_base::binary);
    std::vector<std::pair<std::string, std::string>> metadata;
    metadata.emplace_back(std::make_pair("meta_key1", "meta-value1"));
    metadata.emplace_back(std::make_pair("meta_key2", "meta-value2"));
    ret = client.upload_block_blob_from_stream(container_name, blob_name, fin, metadata).get();
    if (!ret.success())
    {
        std::cout << "Failed to upload blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    fin.close();

    std::ostringstream out_stream;
    ret = client.download_blob_to_stream(container_name, blob_name, 0, 0, out_stream).get();
    if (!ret.success())
    {
        std::cout << "Failed to download blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    else
    {
        std::cout << out_stream.str();
    }

    client.delete_blob(container_name, blob_name).wait();

    return 0;
}
