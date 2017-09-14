#pragma once

#include "get_blob_property_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class get_blob_property_request : public get_blob_property_request_base {
        public:
            get_blob_property_request(const std::string &container, const std::string &blob)
                : m_container(container),
                m_blob(blob) {}

            std::string container() const override {
                return m_container;
            }

            std::string blob() const override {
                return m_blob;
            }

        private:
            std::string m_container;
            std::string m_blob;
        };

    }
}
