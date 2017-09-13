#pragma once

#include <string>
#include <vector>

#include "storage_EXPORTS.h"

#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
namespace storage {

class put_block_list_request_base : public blob_request_base {
public:
    enum class block_type {
        committed,
        uncommitted,
        latest
    };

    class block_item {
    public:
        std::string id;
        block_type type;
    };

    virtual std::string container() const = 0;
    virtual std::string blob() const = 0;

    virtual std::vector<block_item> block_list() const = 0;
    virtual std::vector<std::pair<std::string, std::string>> metadata() const = 0;

    //virtual unsigned int content_length() const = 0;
    virtual std::string content_md5() const { return std::string(); }

    virtual std::string ms_blob_cache_control() const { return std::string(); }
    virtual std::string ms_blob_content_disposition() const { return std::string(); }
    virtual std::string ms_blob_content_encoding() const { return std::string(); }
    virtual std::string ms_blob_content_language() const { return std::string(); }
    virtual std::string ms_blob_content_md5() const { return std::string(); }
    virtual std::string ms_blob_content_type() const { return std::string(); }

    //virtual std::map<std::string, std::string> ms_meta() const {};

    AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
};

//AZURE_STORAGE_API void build_request(const storage_account &a, const put_blob_request_base &r, http_base &h);

}
}
