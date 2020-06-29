#pragma once

#include "adls_request_base.h"

namespace azure { namespace storage_adls {

    class rename_file_request final : public adls_request_base
    {
    public:
        rename_file_request(std::string source_filesystem, std::string source_path, std::string destination_filesystem, std::string destination_path) : m_source_filesystem(std::move(source_filesystem)), m_source_path(std::move(source_path)), m_destination_filesystem(std::move(destination_filesystem)), m_destination_path(std::move(destination_path)) {}

        void build_request(const storage_account& account, http_base& http) const override;
    private:
        std::string m_source_filesystem;
        std::string m_source_path;
        std::string m_destination_filesystem;
        std::string m_destination_path;
    };

}}  // azure::storage_adls