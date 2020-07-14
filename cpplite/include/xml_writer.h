#pragma once

#include <string>
#include <vector>

#include "common.h"
#include "put_block_list_request_base.h"

namespace azure {  namespace storage_lite {

    class xml_writer
    {
    public:
        static std::string write_block_list(const std::vector<put_block_list_request_base::block_item> &items)
        {
            std::string xml;
            xml.append("<?xml version=\"1.0\" encoding=\"utf-8\"?>");
            xml.append("<BlockList>");

            for (const auto &b : items)
            {
                switch (b.type)
                {
                case put_block_list_request_base::block_type::committed:
                    xml.append("<Committed>");
                    break;
                case put_block_list_request_base::block_type::uncommitted:
                    xml.append("<Uncommitted>");
                    break;
                case put_block_list_request_base::block_type::latest:
                    xml.append("<Latest>");
                    break;
                }

                xml.append(b.id);

                switch (b.type)
                {
                case put_block_list_request_base::block_type::committed:
                    xml.append("</Committed>");
                    break;
                case put_block_list_request_base::block_type::uncommitted:
                    xml.append("</Uncommitted>");
                    break;
                case put_block_list_request_base::block_type::latest:
                    xml.append("</Latest>");
                    break;
                }
            }

            xml.append("</BlockList>");
            return xml;
        }
    };

}}
