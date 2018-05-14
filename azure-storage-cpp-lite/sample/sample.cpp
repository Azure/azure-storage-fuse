
#include <string>
#include <chrono>
#include <thread>
#include <assert.h>

#include "storage_credential.h"
#include "storage_account.h"
#include "blob/blob_client.h"


using namespace microsoft_azure::storage;

void checkstatus()
{
    if(errno == 0)
    {
        printf("Success\n");
    }
    else
    {
        printf("Fail\n");
    }
}

int main()
{
    std::string account_name = "jasontesteu2";
    std::string account_key = "3dcKVnUUsnmK01SiNHBFOkgxh3U61/EjFlHkiwYvzmWd5VLeo6CWzhJDaX9RNp51/1TEFWaBiRHdIUNUhLW4cQ==";
    std::shared_ptr<storage_credential>  cred = std::make_shared<shared_key_credential>(account_name, account_key);
    std::shared_ptr<storage_account> account = std::make_shared<storage_account>(account_name, cred, false);
    auto bC = std::make_shared<blob_client>(account, 10);
    //auto f1 = bc.list_containers("");
    //f1.wait();
    //
    std::string containerName = "jasontest1";
    std::string blobName = "test.txt";
    std::string destContainerName = "jasontest1";
    std::string destBlobName = "test.txt.copy";
    std::string uploadFileName = "test.txt";
    std::string downloadFileName = "download.txt";

    bool exists = true;
    blob_client_wrapper bc(bC);
 
    exists = bc.container_exists(containerName);

    if(!exists)
    {
        bc.create_container(containerName);
        assert(errno == 0);
    }

    assert(errno == 0);
    exists = bc.blob_exists(containerName, "testsss.txt");
    assert(errno == 0);
    assert(!exists);
    std::cout <<"Start upload Blob: " << blobName << std::endl;
    bc.upload_file_to_blob(uploadFileName, containerName, blobName);
    std::cout <<"Error upload Blob: " << errno << std::endl;
    assert(errno == 0);

    exists = bc.blob_exists(containerName, blobName);
    assert(errno == 0);
    assert(exists);

    auto blobProperty = bc.get_blob_property(containerName, blobName);
    assert(errno == 0);
    std::cout <<"Size of BLob: " << blobProperty.size << std::endl;

    auto blobs = bc.list_blobs_hierarchical(containerName, "/", "", "");
    std::cout <<"Size of BLobs: " << blobs.blobs.size() << std::endl;
    std::cout <<"Error Size of BLobs: " << errno << std::endl;
    assert(errno == 0);

    time_t last_modified;
    bc.download_blob_to_file(containerName, blobName, downloadFileName, last_modified);
    std::cout <<"Download Blob done: " << errno << std::endl;
    assert(errno == 0);

    exists = bc.container_exists(destContainerName);

    if(!exists)
    {
        bc.create_container(destContainerName);
        assert(errno == 0);
    }

    // copy blob 
    bc.start_copy(containerName, blobName, destContainerName, destBlobName);
    auto property = bc.get_blob_property(destContainerName, destBlobName);
    std::cout << "Copy status: " << property.copy_status <<std::endl;
    exists = bc.blob_exists(destContainerName, destBlobName);
    assert(errno == 0);
    assert(exists);

    bc.delete_blob(containerName, blobName);
    bc.delete_blob(destContainerName, destBlobName);
    assert(errno == 0);
    exists = bc.blob_exists(containerName, blobName);
    assert(errno == 0);
    assert(!exists);
    //bc.delete_container(containerName);
    //assert(errno == 0);
    //std::this_thread::sleep_for(std::chrono::seconds(5));
}
