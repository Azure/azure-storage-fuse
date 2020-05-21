#pragma once

#include "append_block_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class append_block_request : public append_block_request_base {
        public:
            append_block_request(const std::string &container, const std::string &blob)
                : m_container(container),
                m_blob(blob),
                m_content_length(0) {}

            std::string container() const override {
                return m_container;
            }

            std::string blob() const override {
                return m_blob;
            }

            unsigned int content_length() const override {
                return m_content_length;
            }

            append_block_request &set_content_length(unsigned int content_length) {
                m_content_length = content_length;
                return *this;
            }

        private:
            std::string m_container;
            std::string m_blob;

            unsigned int m_content_length;
        };

    }
}
