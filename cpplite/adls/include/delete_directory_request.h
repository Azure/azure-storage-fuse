#pragma once

#include "adls_request_base.h"

namespace azure { namespace storage_adls {

    class delete_directory_request final : public adls_request_base
    {
    public:
        delete_directory_request(std::string filesystem, std::string directory, std::string continuation) : m_filesystem(std::move(filesystem)), m_directory(std::move(directory)), m_continuation(std::move(continuation)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_filesystem;
        std::string m_directory;
        std::string m_continuation;
    };

}}  // azure::storage_adls