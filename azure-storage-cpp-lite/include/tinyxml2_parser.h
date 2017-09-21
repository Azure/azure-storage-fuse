#pragma once

#include "tinyxml2.h"

#include "storage_EXPORTS.h"

#include "storage_outcome.h"
#include "xml_parser_base.h"

namespace microsoft_azure {
namespace storage {

class tinyxml2_parser : public xml_parser_base {
public:
    AZURE_STORAGE_API storage_error parse_storage_error(const std::string &xml) const override;

    //AZURE_STORAGE_API list_containers_response parse_list_containers_response(const std::string &xml, std::vector<list_containers_item> &items) const override;

    AZURE_STORAGE_API list_containers_response parse_list_containers_response(const std::string &xml) const override;

    AZURE_STORAGE_API list_blobs_response parse_list_blobs_response(const std::string &xml) const override;

    AZURE_STORAGE_API list_blobs_hierarchical_response parse_list_blobs_hierarchical_response(const std::string &xml) const override;

    AZURE_STORAGE_API get_block_list_response parse_get_block_list_response(const std::string &xml) const override;

    AZURE_STORAGE_API get_page_ranges_response parse_get_page_ranges_response(const std::string &xml) const override;

private:
    AZURE_STORAGE_API std::string parse_text(tinyxml2::XMLElement *ele, const std::string &name) const;

    AZURE_STORAGE_API unsigned long long parse_long(tinyxml2::XMLElement *ele, const std::string &name) const;

    AZURE_STORAGE_API list_containers_item parse_list_containers_item(tinyxml2::XMLElement *ele) const;

    AZURE_STORAGE_API list_blobs_item parse_list_blobs_item(tinyxml2::XMLElement *ele) const;

    AZURE_STORAGE_API std::vector<std::pair<std::string, std::string>> parse_blob_metadata(tinyxml2::XMLElement *ele) const;

    AZURE_STORAGE_API list_blobs_hierarchical_item parse_list_blobs_hierarchical_item(tinyxml2::XMLElement *ele, bool is_directory) const;

    AZURE_STORAGE_API get_block_list_item parse_get_block_list_item(tinyxml2::XMLElement *ele) const;

    AZURE_STORAGE_API get_page_ranges_item parse_get_page_ranges_item(tinyxml2::XMLElement *ele) const;
};

}
}
