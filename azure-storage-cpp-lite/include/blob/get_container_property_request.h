#pragma once

#include "get_container_property_request_base.h"

namespace microsoft_azure {
    namespace storage {

        class get_container_property_request : public get_container_property_request_base {
        public:
            get_container_property_request(const std::string &container)
                : m_container(container)
            {}

            std::string container() const override {
                return m_container;
            }

        private:
            std::string m_container;
        };
    }
}
