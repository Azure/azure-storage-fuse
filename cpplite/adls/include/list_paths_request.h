#pragma once

#include "adls_request_base.h"
#include "set_access_control_request.h"

namespace azure { namespace storage_adls {

    struct list_paths_item
    {
        std::string name;
        std::string etag;
        uint64_t content_length;
        std::string last_modified;
        access_control acl;
        bool is_directory;
    };

    struct list_paths_result
    {
        std::vector<list_paths_item> paths;
        std::string continuation_token;
    };

    class list_paths_request final : public adls_request_base
    {
    public:
        list_paths_request(std::string filesystem, std::string directory, bool recursive, std::string continuation, int max_results) : m_filesystem(std::move(filesystem)), m_directory(std::move(directory)), m_recursive(recursive), m_continuation(std::move(continuation)), m_max_results(max_results) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_directory;
        bool m_recursive;
        std::string m_continuation;
        int m_max_results;
    };

}}  // azure::storage_adls