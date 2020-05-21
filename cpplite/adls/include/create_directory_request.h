#pragma once

#include "adls_request_base.h"

namespace azure { namespace storage_adls {

    class create_directory_request final : public adls_request_base
    {
    public:
        create_directory_request(std::string filesystem, std::string directory) : m_filesystem(std::move(filesystem)), m_directory(std::move(directory)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_directory;
    };

}}  // azure::storage_adls