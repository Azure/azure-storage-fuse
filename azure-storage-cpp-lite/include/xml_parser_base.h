#pragma once

#include <string>
#include <vector>

#include "common.h"
#include "list_containers_request_base.h"
#include "list_blobs_request_base.h"
#include "get_block_list_request_base.h"
#include "get_page_ranges_request_base.h"

namespace microsoft_azure {
namespace storage {

class xml_parser_base {
public:
    virtual storage_error parse_storage_error(const std::string &) const = 0;

    template<typename RESPONSE_TYPE>
    RESPONSE_TYPE parse_response(const std::string &) const {}

    //virtual list_containers_response parse_list_containers_response(const std::string &xml, std::vector<list_containers_item> &items) const = 0;

    virtual list_containers_response parse_list_containers_response(const std::string &xml) const = 0;

    virtual list_blobs_response parse_list_blobs_response(const std::string &xml) const = 0;

    virtual list_blobs_hierarchical_response parse_list_blobs_hierarchical_response(const std::string &xml) const = 0;

    virtual get_block_list_response parse_get_block_list_response(const std::string &xml) const = 0;

    virtual get_page_ranges_response parse_get_page_ranges_response(const std::string &xml) const = 0;
};

template<>
inline list_containers_response xml_parser_base::parse_response<list_containers_response>(const std::string &xml) const {
    return parse_list_containers_response(xml);
}

template<>
inline list_blobs_response xml_parser_base::parse_response<list_blobs_response>(const std::string &xml) const {
    return parse_list_blobs_response(xml);
}

template<>
inline list_blobs_hierarchical_response xml_parser_base::parse_response<list_blobs_hierarchical_response>(const std::string &xml) const {
    return parse_list_blobs_hierarchical_response(xml);
}

template<>
inline get_block_list_response xml_parser_base::parse_response<get_block_list_response>(const std::string &xml) const {
    return parse_get_block_list_response(xml);
}

template<>
inline get_page_ranges_response xml_parser_base::parse_response<get_page_ranges_response>(const std::string &xml) const {
    return parse_get_page_ranges_response(xml);
}

}
}
