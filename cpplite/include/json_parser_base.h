#pragma once

#include <string>

#include "common.h"

namespace azure {  namespace storage_lite {

    class json_parser_base
    {
    public:
        virtual ~json_parser_base() = 0;

        template<typename RESPONSE_TYPE>
        RESPONSE_TYPE parse_response(const std::string &) const { return RESPONSE_TYPE(); }
    };

    inline json_parser_base::~json_parser_base() {}

}}   // azure::storage_lite
