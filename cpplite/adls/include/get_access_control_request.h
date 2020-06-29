#pragma once

#include "adls_request_base.h"
#include "set_access_control_request.h"

namespace azure { namespace storage_adls {

    class get_access_control_request final : public adls_request_base
    {
    public:
        get_access_control_request(std::string filesystem, std::string path) : m_filesystem(std::move(filesystem)), m_path(std::move(path)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_path;
    };

}}  // azure::storage_adls