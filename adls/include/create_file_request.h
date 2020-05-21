#pragma once

#include "adls_request_base.h"

namespace azure { namespace storage_adls {

    class create_file_request final : public adls_request_base
    {
    public:
        create_file_request(std::string filesystem, std::string file) : m_filesystem(std::move(filesystem)), m_file(std::move(file)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_file;
    };

}}  // azure::storage_adls