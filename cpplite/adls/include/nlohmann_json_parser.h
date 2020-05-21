#pragma once

#include <string>
#include <vector>

#include "json_parser_base.h"
#include "list_paths_request.h"

namespace azure { namespace storage_adls {

    class nlohmann_json_parser final : public azure::storage_lite::json_parser_base
    {
    public:
        static std::vector<list_paths_item> parse_list_paths_response(const std::string& json);
    };

}}  // azure::storage_adls

namespace azure { namespace storage_lite {

    template<>
    inline std::vector<azure::storage_adls::list_paths_item> json_parser_base::parse_response(const std::string& json) const
    {
        return azure::storage_adls::nlohmann_json_parser::parse_list_paths_response(json);
    }

}}  // azure::storage_lite