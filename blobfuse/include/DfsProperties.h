//
// Created by adreed on 2/7/2020.
//

#ifndef BLOBFUSE_GET_DFS_PROPERTIES_REQUEST_H
#define BLOBFUSE_GET_DFS_PROPERTIES_REQUEST_H


#include <string>
#include <adls_request_base.h>
#include <adls_client.h>

using namespace azure::storage_adls;
using namespace azure::storage_lite;

namespace azure { namespace storage_adls {


struct dfs_properties
{
    std::string cache_control;
    std::string content_disposition;
    std::string content_encoding;
    std::string content_language;
    unsigned long long content_length;
    std::string content_type;
    std::string content_md5;
    std::string etag;
    std::string resource_type;
    std::vector<std::pair<std::string, std::string>> metadata;
    time_t last_modified;
    std::string owner;
    std::string group;
    std::string permissions;
    std::string acl;
};

class get_dfs_properties_request : public adls_request_base
{
public:
    get_dfs_properties_request(std::string filesystem, std::string path) :
        m_filesystem(std::move(filesystem)),
        m_path(std::move(path)) {}

    void build_request(const storage_account& account, http_base& http) const;

private:
    std::string m_filesystem;
    std::string m_path;
};


class adls_client_ext : public adls_client
{
public:
    
    adls_client_ext(
                std::shared_ptr<storage_account> account, 
                int max_concurrency, 
                bool exception_enabled = true) :
                adls_client(account, max_concurrency, exception_enabled),
                maxConcurrency(max_concurrency)
    {

    }

    /// <summary>
    /// Gets the full properties for a path.
    /// </summary>
    /// <param name="filesystem">The filesystem name.</param>
    /// <param name="path">The path.</param>
    AZURE_STORAGE_API dfs_properties get_dfs_path_properties(const std::string& filesystem, const std::string& path);
    AZURE_STORAGE_ADLS_API void append_data_from_file(const std::string &src_file, const std::string& filesystem, const std::string& file, const std::vector<std::pair<std::string, std::string>>& properties = std::vector<std::pair<std::string, std::string>>());
     

    /// <summary>
    /// Returns 1 if path exists false otherwise.
    /// </summary>
    /// <param name="filesystem">The filesystem name.</param>
    /// <param name="path">The path.</param>
    AZURE_STORAGE_API int adls_exists(const std::string& filesystem, const std::string& path);

        
    template<class RET, class FUNC>
    RET blob_client_adaptor_ext(FUNC func);

    private:
        unsigned int maxConcurrency;
};

}}


#endif //BLOBFUSE_GET_DFS_PROPERTIES_REQUEST_H
