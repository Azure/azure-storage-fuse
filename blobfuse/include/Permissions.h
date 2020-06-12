//
// Created by adreed on 2/6/2020.
//

#ifndef BLOBFUSE_PERMISSIONS_H
#define BLOBFUSE_PERMISSIONS_H

#include <string>
#include <blobfuse_constants.h>
#include <set_access_control_request.h>

using namespace azure::storage_adls;

std::string modeToString(mode_t mode);
mode_t aclToMode(access_control acl);

#endif //BLOBFUSE_PERMISSIONS_H
