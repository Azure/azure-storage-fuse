#pragma once

#include <map>
#include <string>
#include <vector>

#include "storage_EXPORTS.h"

#include "common.h"
#include "http_base.h"
#include "storage_account.h"
#include "storage_request_base.h"

namespace microsoft_azure {
namespace storage {

class list_blobs_request_base : public blob_request_base {
public:
    enum include {
        unspecifies = 0x0,
        snapshots = 0x1,
        metadata = 0x2,
        uncommittedblobs = 0x4,
        copy = 0x8
    };

    virtual std::string container() const = 0;
    virtual std::string prefix() const { return std::string(); }
    virtual std::string delimiter() const { return std::string(); }
    virtual std::string marker() const { return std::string(); }
    virtual int maxresults() const { return 0; }
    virtual include includes() const { return include::unspecifies; }

    AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
};

class list_blobs_item {
public:
    std::string name;
    std::string snapshot;
    std::string last_modified;
    std::string etag;
    unsigned long long content_length;
    std::string content_encoding;
    std::string content_type;
    std::string content_md5;
    std::string content_language;
    std::string cache_control;
    lease_status status;
    lease_state state;
    lease_duration duration;
};

class list_blobs_response {
public:
    std::string ms_request_id;
    std::vector<list_blobs_item> blobs;
    std::string next_marker;
};


class list_blobs_hierarchical_request_base : public blob_request_base {
public:

    virtual std::string container() const = 0;
    virtual std::string prefix() const { return std::string(); }
    virtual std::string delimiter() const { return std::string(); }
    virtual std::string marker() const { return std::string(); }
    virtual int maxresults() const { return 0; }
    virtual list_blobs_request_base::include includes() const { return list_blobs_request_base::include::unspecifies; }

    AZURE_STORAGE_API void build_request(const storage_account &a, http_base &h) const override;
};

class list_blobs_hierarchical_item {
public:
    std::string name;
    std::string snapshot;
    std::string last_modified;
    std::string etag;
    unsigned long long content_length;
    std::string content_encoding;
    std::string content_type;
    std::string content_md5;
    std::string content_language;
    std::string cache_control;
    lease_status status;
    lease_state state;
    lease_duration duration;
    std::string copy_status;
    std::vector<std::pair<std::string, std::string>> metadata;
    bool is_directory;
};

class list_blobs_hierarchical_response {
public:
    std::string ms_request_id;
    std::vector<list_blobs_hierarchical_item> blobs;
    std::string next_marker;
};

}
}

