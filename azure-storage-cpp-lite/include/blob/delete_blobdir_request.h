#pragma once

#include "delete_blob_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class delete_blobdir_request final : public delete_blob_request_base {
        public:
            delete_blobdir_request(const std::string &container, const std::string &blob)
                : m_container(container),
                m_blob(blob) {}

            std::string container() const override {
                return m_container;
            }

            std::string blob() const override {
                return m_blob;
            }

            delete_snapshots ms_delete_snapshots() const override {
                return delete_snapshots::unspecified;
            }

        private:
            std::string m_container;
            std::string m_blob;
        };

    }
}
