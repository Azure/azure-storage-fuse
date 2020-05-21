#pragma once

#include <string>

namespace azure {  namespace storage_lite {

        class storage_account;
        class http_base;

        class storage_request_base
        {
        public:
            virtual ~storage_request_base() = default;
            // TODO: create request ID for each request for future debugging purposes.
            virtual std::string ms_client_request_id() const { return std::string(); }

            virtual void build_request(const storage_account &a, http_base &h) const = 0;
        };

        class blob_request_base : public storage_request_base
        {
        public:
            virtual unsigned int timeout() const { return 0; }
            virtual std::string if_modified_since() const { return std::string(); }
            virtual std::string if_match() const { return std::string(); }
            virtual std::string if_none_match() const { return std::string(); }
            virtual std::string if_unmodified_since() const { return std::string(); }
            virtual std::string ms_lease_id() const { return std::string(); }
        };

        class table_request_base : public storage_request_base {};

        class queue_request_base : public storage_request_base {};

        class file_request_base : public storage_request_base {};

}}
