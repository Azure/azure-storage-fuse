#pragma once

#include "storage_request_base.h"
#include "storage_account.h"
#include "constants.h"

namespace azure { namespace storage_adls {

    using storage_account = azure::storage_lite::storage_account;
    using http_base = azure::storage_lite::http_base;

    class adls_request_base : public azure::storage_lite::storage_request_base
    {
    };

}}  // azure::storage_adls