#include "tinyxml2_parser.h"

#include "utility.h"

namespace microsoft_azure {
namespace storage {

std::string tinyxml2_parser::parse_text(tinyxml2::XMLElement *ele, const std::string &name) const {
    std::string text;
    ele = ele->FirstChildElement(name.data());
    if (ele && ele->FirstChild()) {
        text = ele->FirstChild()->ToText()->Value();
    }

    return text;
}

unsigned long long tinyxml2_parser::parse_long(tinyxml2::XMLElement *ele, const std::string &name) const {
    unsigned long long result = 0;

    std::string text = parse_text(ele, name);
    if (!text.empty()) {
        std::istringstream iss(text);
        iss >> result;
    }

    return result;
}

storage_error tinyxml2_parser::parse_storage_error(const std::string &xml) const {
    storage_error error;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XMLError::XML_SUCCESS) {
        auto xerror = xdoc.FirstChildElement("Error");
        error.code_name = parse_text(xerror, "Code");
        error.message = parse_text(xerror, "Message");
    }

    return error;
}

list_containers_item tinyxml2_parser::parse_list_containers_item(tinyxml2::XMLElement *ele) const {
    list_containers_item item;

    item.name = parse_text(ele, "Name");

    auto xproperty = ele->FirstChildElement("Properties");
    item.etag = parse_text(xproperty, "Etag");
    item.last_modified = parse_text(xproperty, "Last-Modified");
    item.status = parse_lease_status(parse_text(xproperty, "LeaseStatus"));
    item.state = parse_lease_state(parse_text(xproperty, "LeaseState"));
    item.duration = parse_lease_duration(parse_text(xproperty, "LeaseDuration"));

    //parse_metadata

    return item;
}

/*list_containers_response tinyxml2_parser::parse_list_containers_response(const std::string &xml, std::vector<list_containers_item> &items) const {
    list_containers_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("EnumerationResults");
        response.next_marker = parse_text(xresults, "NextMarker");
        auto xcontainers = xresults->FirstChildElement("Containers");
        auto xcontainer = xcontainers->FirstChildElement("Container");
        while (xcontainer) {
            items.push_back(parse_list_containers_item(xcontainer));
            xcontainer = xcontainer->NextSiblingElement("Container");
        }
    }

    return response;
}*/

list_containers_response tinyxml2_parser::parse_list_containers_response(const std::string &xml) const {
    list_containers_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("EnumerationResults");
        response.next_marker = parse_text(xresults, "NextMarker");
        auto xitems = xresults->FirstChildElement("Containers");
        auto xitem = xitems->FirstChildElement("Container");
        while (xitem) {
            response.containers.push_back(parse_list_containers_item(xitem));
            xitem = xitem->NextSiblingElement("Container");
        }
    }

    return response;
}

list_blobs_item tinyxml2_parser::parse_list_blobs_item(tinyxml2::XMLElement *ele) const {
    list_blobs_item item;

    item.name = parse_text(ele, "Name");

    auto xproperty = ele->FirstChildElement("Properties");
    item.etag = parse_text(xproperty, "Etag");
    item.last_modified = parse_text(xproperty, "Last-Modified");
    item.cache_control = parse_text(xproperty, "Cache-Control");
    item.content_encoding = parse_text(xproperty, "Content-Encoding");
    item.content_language = parse_text(xproperty, "Content-Language");
    item.content_type = parse_text(xproperty, "Content-Type");
    item.content_md5 = parse_text(xproperty, "Content-MD5");
    item.content_length = parse_long(xproperty, "Content-Length");
    item.status = parse_lease_status(parse_text(xproperty, "LeaseStatus"));
    item.state = parse_lease_state(parse_text(xproperty, "LeaseState"));
    item.duration = parse_lease_duration(parse_text(xproperty, "LeaseDuration"));

    //parse_metadata

    return item;
}

list_blobs_response tinyxml2_parser::parse_list_blobs_response(const std::string &xml) const {
    list_blobs_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("EnumerationResults");
        response.next_marker = parse_text(xresults, "NextMarker");
        auto xitems = xresults->FirstChildElement("Blobs");
        auto xitem = xitems->FirstChildElement("Blob");
        while (xitem) {
            response.blobs.push_back(parse_list_blobs_item(xitem));
            xitem = xitem->NextSiblingElement("Blob");
        }
    }

    return response;
}

std::vector<std::pair<std::string, std::string>> tinyxml2_parser::parse_blob_metadata(tinyxml2::XMLElement *ele) const {
    std::vector<std::pair<std::string, std::string>> metadata;
    tinyxml2::XMLElement *current = ele->FirstChildElement();
    while (current)
    {
        std::string name(current->Name());
        std::string value(current->GetText());
        metadata.push_back(make_pair(name, value));
        current = current->NextSiblingElement();
    }
    return metadata;
}

list_blobs_hierarchical_item tinyxml2_parser::parse_list_blobs_hierarchical_item(tinyxml2::XMLElement *ele, bool is_directory) const {
    list_blobs_hierarchical_item item;

    item.name = parse_text(ele, "Name");
    item.is_directory = is_directory;
    if (!is_directory)
    {
        auto xproperty = ele->FirstChildElement("Properties");
        item.etag = parse_text(xproperty, "Etag");
        item.last_modified = parse_text(xproperty, "Last-Modified");
        item.cache_control = parse_text(xproperty, "Cache-Control");
        item.content_encoding = parse_text(xproperty, "Content-Encoding");
        item.content_language = parse_text(xproperty, "Content-Language");
        item.content_type = parse_text(xproperty, "Content-Type");
        item.content_md5 = parse_text(xproperty, "Content-MD5");
        item.copy_status = parse_text(xproperty, "CopyStatus");
        item.content_length = parse_long(xproperty, "Content-Length");
        item.status = parse_lease_status(parse_text(xproperty, "LeaseStatus"));
        item.state = parse_lease_state(parse_text(xproperty, "LeaseState"));
        item.duration = parse_lease_duration(parse_text(xproperty, "LeaseDuration"));
        auto xmetadata = ele->FirstChildElement("Metadata");
        if (xmetadata)
        {
            item.metadata = parse_blob_metadata(xmetadata);
        }
    }
    //parse_metadata

    return item;
}

list_blobs_hierarchical_response tinyxml2_parser::parse_list_blobs_hierarchical_response(const std::string &xml) const {
    list_blobs_hierarchical_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("EnumerationResults");
        response.next_marker = parse_text(xresults, "NextMarker");
        auto xitems = xresults->FirstChildElement("Blobs");
        auto xitem = xitems->FirstChildElement("Blob");
        while (xitem) {
            response.blobs.push_back(parse_list_blobs_hierarchical_item(xitem, false));
            xitem = xitem->NextSiblingElement("Blob");
        }

        auto xdir = xitems->FirstChildElement("BlobPrefix");
        while (xdir) {
            response.blobs.push_back(parse_list_blobs_hierarchical_item(xdir, true));
            xdir = xdir->NextSiblingElement("BlobPrefix");
        }
    }

    return response;
}


get_block_list_item tinyxml2_parser::parse_get_block_list_item(tinyxml2::XMLElement *ele) const {
    get_block_list_item item;

    item.name = parse_text(ele, "Name");
    item.size = parse_long(ele, "Size");

    return item;
}

get_block_list_response tinyxml2_parser::parse_get_block_list_response(const std::string &xml) const {
    get_block_list_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("BlockList");
        auto xitems = xresults->FirstChildElement("CommittedBlocks");
        auto xitem = xitems->FirstChildElement("Block");
        while (xitem) {
            response.committed.push_back(parse_get_block_list_item(xitem));
            xitem = xitem->NextSiblingElement("Block");
        }

        xitems = xresults->FirstChildElement("UncommittedBlocks");
        xitem = xitems->FirstChildElement("Block");
        while (xitem) {
            response.uncommitted.push_back(parse_get_block_list_item(xitem));
            xitem = xitem->NextSiblingElement("Block");
        }
    }

    return response;
}

get_page_ranges_item tinyxml2_parser::parse_get_page_ranges_item(tinyxml2::XMLElement *ele) const {
    get_page_ranges_item item;

    item.start = parse_long(ele, "Start");
    item.end = parse_long(ele, "End");

    return item;
}

get_page_ranges_response tinyxml2_parser::parse_get_page_ranges_response(const std::string &xml) const {
    get_page_ranges_response response;

    tinyxml2::XMLDocument xdoc;
    if (xdoc.Parse(xml.data(), xml.size()) == tinyxml2::XML_SUCCESS) {
        auto xresults = xdoc.FirstChildElement("PageList");
        auto xitem = xresults->FirstChildElement("PageRange");
        while (xitem) {
            response.pagelist.push_back(parse_get_page_ranges_item(xitem));
            xitem = xitem->NextSiblingElement("PageRange");
        }
    }

    return response;
}

}
}
