#pragma once

#include "delete_container_request_base.h"

namespace azure {  namespace storage_lite {

    class delete_container_request final : public delete_container_request_base
    {
    public:
        delete_container_request(const std::string &container)
            : m_container(container) {}

        std::string container() const override {
            return m_container;
        }

    private:
        std::string m_container;
    };

}}  // azure::storage_lite
