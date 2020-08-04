//
// ADLS specific properties
//

#include <DfsProperties.h>
#include <BlobfuseConstants.h>
#include <utility.h>

#include <base64.h>
#include <blob/blob_client.h>
#include <adls_client.h>

#include <storage_errno.h>
#include <sys/stat.h>
#include <iostream>
#include <fstream>

using namespace azure::storage_adls;
using namespace azure::storage_lite;

namespace azure { namespace storage_adls {

void get_dfs_properties_request::build_request(const storage_account &account, http_base &http) const
{
    http.set_method(http_base::http_method::head);

    storage_url url = account.get_url(storage_account::service::adls);
    url.append_path(m_filesystem).append_path(m_path);

    http.set_url(url.to_string());

    storage_headers headers;
    http.add_header(blobfuse_constants::header_user_agent, blobfuse_constants::header_value_user_agent);
    add_ms_header(http, headers, blobfuse_constants::header_ms_date, get_ms_date(date_format::rfc_1123));
    add_ms_header(http, headers, blobfuse_constants::header_ms_version, blobfuse_constants::header_value_storage_version);

    account.credential()->sign_request(*this, http, url, headers);
}

template<class RET, class FUNC>
RET adls_client_ext::blob_client_adaptor_ext(FUNC func)
{
    try
    {
        storage_outcome<RET> result = func().get();

        if (result.success() && !m_exception_enabled)
        {
            errno = 0;
        }
        else if (!result.success())
        {
            int error_code = std::stoi(result.error().code);
            if (m_exception_enabled)
            {
                throw storage_exception(error_code, result.error().code_name, result.error().message);
            }
            else
            {
                errno = error_code;
            }
        }

        return result.response();
    }
    catch (std::exception& e)
    {
        if (m_exception_enabled)
        {
            throw;
        }
        else
        {
            errno = blobfuse_constants::unknown_error;
        }
    }
    return RET();
}

dfs_properties adls_client_ext::get_dfs_path_properties(const std::string &filesystem, const std::string &path) 
{
    auto http = m_blob_client->client()->get_handle();
    dfs_properties props;
    
    if (adls_exists(filesystem, path, http))
    {
        props.cache_control = http->get_response_header(constants::header_cache_control);
        props.content_disposition = http->get_response_header(constants::header_content_disposition);
        props.content_encoding = http->get_response_header(constants::header_content_encoding);
        props.content_language = http->get_response_header(constants::header_content_language);

        std::string cLen = http->get_response_header(constants::header_content_length);
        if (!cLen.empty())
            props.content_length = std::stoull(cLen);

        props.content_type = http->get_response_header(constants::header_content_type);
        props.content_md5 = http->get_response_header(constants::header_content_md5);
        props.etag = http->get_response_header(constants::header_etag);
        props.last_modified = curl_getdate(http->get_response_header(constants::header_last_modified).c_str(), NULL);
        props.resource_type = http->get_response_header(blobfuse_constants::header_ms_resource_type);
        props.owner = http->get_response_header(constants::header_ms_owner);
        props.group = http->get_response_header(constants::header_ms_group);
        props.permissions = http->get_response_header(constants::header_ms_permissions);
        // acl is not returned in this call
              //  props.acl = http->get_response_header(constants::header_ms_acl);

        // props.metadata TODO
        props.metadata = std::vector<std::pair<std::string, std::string>>{};

        std::string runningString, propName;
        std::string temp = http->get_response_header(blobfuse_constants::header_ms_properties);
        std::size_t pos_value = 0;
        
        std::size_t pos = temp.find("=");
        while(pos != std::string::npos) {
            propName = temp.substr(0, pos);
            pos++;

            pos_value = temp.find(",");
            if (pos_value == std::string::npos)
                pos_value = temp.length();
            runningString = temp.substr(pos, (pos_value- pos));

            std::string prop;
            for (unsigned char dc : from_base64(runningString)) {
                prop += dc;
            }

            props.metadata.emplace_back(propName, prop);

            if (pos_value < temp.length()) {
                temp = temp.substr(pos_value + 1);
                pos = temp.find("=");
            } else {
                break;
            }
        }

    }

    return props;
}

off_t get_file_size(const std::string file_name)
{
    off_t size = 0;
    struct stat st;

    if(stat(file_name.c_str(), &st) == 0)
    {
        size = st.st_size;
    }
    return size;
}

void adls_client_ext::append_data_from_file(const std::string &src_file, const std::string& filesystem, const std::string& file, const std::vector<std::pair<std::string, std::string>>& properties)
{
    const long long MAX_BLOB_SIZE = 5242880000000; // 4.77TB 
    const long long MIN_UPLOAD_CHUNK_SIZE = 16 * 1024 * 1024;

    if(src_file.empty() || filesystem.empty() || file.empty())
    {
        errno = invalid_parameters;
        return;
    }
    
    long long file_size = get_file_size(src_file);        
    if (file_size > MAX_BLOB_SIZE) {
        errno = EFBIG;
        return;
    }

    // Upoad to adls is a three step process
    // 1. Create a file place holder
    errno = 0;
    create_file(filesystem, file);
    if (errno) {
        return;
    }

    if (file_size == 0) {
        // This is an empty file nothing more to be done here
        return;
    }

    // 2. Upload all the blocks
    long long offset = 0;

    if(file_size <= (64 * 1024 * 1024)){
        // upto 64 MB file just upload in one shot
        offset = file_size;
        std::ifstream ifs(src_file, std::ios::in | std::ios::binary);

        if(!ifs)
        {
            syslog(LOG_DEBUG, "Failed to open the input stream in append_data_from_file.  errno = %d, sourcePath = %s.", errno, src_file.c_str());
            errno = unknown_error;
            return;
        }

        append_data_from_stream(filesystem, file, 0, ifs, 0);

        if (errno) {
            syslog(LOG_DEBUG, "Failed to upload the input stream in append_data_from_file.  errno = %d, sourcePath = %s.", errno, src_file.c_str());
            ifs.close();
            return;
        }

        ifs.close();
    } else {
        // File is bigger so we need to split this up
        long long block_size = MIN_UPLOAD_CHUNK_SIZE;
        if(file_size > (50000 * MIN_UPLOAD_CHUNK_SIZE))
        {
            long long min_block = file_size / 50000; 
            int remainder = min_block % 4*1024*1024;
            min_block += 4*1024*1024 - remainder;
            block_size = min_block < MIN_UPLOAD_CHUNK_SIZE ? MIN_UPLOAD_CHUNK_SIZE : min_block;
        }

        std::deque<std::future<int>> task_list;
        int result = 0;

        for(offset = 0; offset < file_size; offset += block_size)
        {
            // control the number of submitted jobs.
            while(task_list.size() > maxConcurrency)
            {
                auto r = task_list.front().get();
                task_list.pop_front();
                if (0 == result) {
                    result = r;
                }
            }
            if (0 != result) {
                break;
            }

            auto single_put = std::async(std::launch::async, [this, filesystem, file, src_file, file_size, offset, block_size](){
                std::ifstream ifs(src_file, std::ios::in | std::ios::binary);
                if(!ifs)
                {
                    syslog(LOG_DEBUG, "Failed to open the input stream in append_data_from_file.  errno = %d, sourcePath = %s.", errno, src_file.c_str());
                    errno = unknown_error;
                    return unknown_error;
                }
                ifs.seekg(offset);

                if (block_size >= (file_size - offset))
                    append_data_from_stream(filesystem, file, offset, ifs, 0);
                else
                    append_data_from_stream(filesystem, file, offset, ifs, block_size);

                int result = errno;
                ifs.close();           
                return result; 
            });
            task_list.push_back(std::move(single_put));
        } 

        // Wait for all async threads to finish the upload
        for(auto &task: task_list)
        {
            const auto r = task.get();
            if(0 == result)
            {
                result = r;
            }
        } 
    }

    if (offset < file_size) {
        syslog(LOG_DEBUG, "Failed to upload data in append_data_from_file.  errno = %d, sourcePath = %s.", errno, src_file.c_str());
        return;
    }

    // 3. Flush the data to persist it
    flush_data(filesystem, file, file_size);

    // 4. Metadata for file is yet not updated so lets update that
    if (properties.size() > 0)
        set_file_properties(filesystem, file, properties);

    return;
}
    
/// Method that calls the dfs endpoint to find out if the pathe exists
/// returns 0 if there is no path returns 1 if there is path
int adls_client_ext::adls_exists(const std::string &filesystem, const std::string &path, std::shared_ptr<azure::storage_lite::CurlEasyRequest> http) 
{
    int exists = 0;
    if (http == NULL)
        http = m_blob_client->client()->get_handle();

    auto request = std::make_shared<get_dfs_properties_request>(filesystem, path);
    auto async_func = std::bind(&azure::storage_lite::async_executor<void>::submit, m_account, request, http, m_context);
    blob_client_adaptor_ext<void>(async_func);   
    
    if (success())
    {
        exists=1;
    }
    if (errno == 404)
    {
        exists=0;
    } 
    return exists;
}


}}
