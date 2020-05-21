#pragma once

#include "adls_request_base.h"

namespace azure { namespace storage_adls {

    struct access_control
    {
        std::string owner;
        std::string group;
        std::string permissions;
        std::string acl;
    };

    class set_access_control_request final : public adls_request_base
    {
    public:
        set_access_control_request(std::string filesystem, std::string path, access_control acl) : m_filesystem(std::move(filesystem)), m_path(std::move(path)), m_acl(std::move(acl)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_path;
        access_control m_acl;
    };

}}  // azure::storage_adls