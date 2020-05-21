#pragma once
/* common errors*/
const int invalid_parameters = 1200;
/* client level*/
const int client_init_fail = 1300;
const int client_already_init = 1301;
const int client_not_init = 1302;
/* container level*/
const int container_already_exists = 1400;
const int container_not_exists = 1401;
const int container_name_invalid = 1402;
const int container_create_fail = 1403;
const int container_delete_fail = 1404;
/* blob level*/
const int blob_already_exists = 1500;
const int blob_not_exists = 1501;
const int blob_name_invalid = 1502;
const int blob_delete_fail = 1503;
const int blob_list_fail = 1504;
const int blob_copy_fail = 1505;
const int blob_no_content_range = 1506;
const int blob_too_big = 1507;
/* unknown error*/
const int unknown_error = 1600;
