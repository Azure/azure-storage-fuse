#include "json.hpp"
#include "nlohmann_json_parser.h"
#include "set_access_control_request.h"

namespace azure { namespace storage_adls {

    std::vector<list_paths_item> nlohmann_json_parser::parse_list_paths_response(const std::string& response)
    {
        auto json_object = nlohmann::json::parse(response);
        std::vector<azure::storage_adls::list_paths_item> paths;

        for (const auto& path_element : json_object["paths"])
        {
            list_paths_item path_item;
            path_item.name = path_element["name"].get<std::string>();
            path_item.content_length = std::stoull(path_element["contentLength"].get<std::string>());
            path_item.etag = path_element["etag"].get<std::string>();
            path_item.last_modified = path_element["lastModified"].get<std::string>();
            path_item.acl.owner = path_element["owner"].get<std::string>();
            path_item.acl.group = path_element["group"].get<std::string>();
            path_item.acl.permissions = path_element["permissions"].get<std::string>();
            path_item.is_directory = path_element.count("isDirectory") && path_element["isDirectory"].get<std::string>() == "true";
            paths.emplace_back(std::move(path_item));
        }

        return paths;
    }

}}  // azure::storage_adls
