#pragma once

#include "delete_container_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class delete_container_request : public delete_container_request_base {
        public:
            delete_container_request(const std::string &container)
                : m_container(container) {}

            std::string container() const override {
                return m_container;
            }

        private:
            std::string m_container;
        };

    }
}
