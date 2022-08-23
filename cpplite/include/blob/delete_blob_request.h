#pragma once

#include "delete_blob_request_base.h"

namespace azure { namespace storage_lite {

    class delete_blob_request final : public delete_blob_request_base
    {
    public:
        delete_blob_request(const std::string &container, const std::string &blob, bool delete_snapshots_only = false, bool is_directory = false)
            : m_container(container),
            m_blob(blob),
            m_delete_snapshots_only(delete_snapshots_only),
            m_is_directory(is_directory) {}

        std::string container() const override
        {
            return m_container;
        }

        std::string blob() const override
        {
            return m_blob;
        }

        delete_snapshots ms_delete_snapshots() const override
        {
            if (m_is_directory) {
                return delete_snapshots::unspecified;
            }
            if (m_delete_snapshots_only) {
                return delete_snapshots::only;
            }
            else {
                return delete_snapshots::include;
            }
        }

    private:
        std::string m_container;
        std::string m_blob;
        bool m_delete_snapshots_only;
        bool m_is_directory;
    };

}}  // azure::storage_lite
