#include <Permissions.h>

std::string modeToString(mode_t mode) {
    //The format for the value x-ms-acl is user::rwx,group::rwx,mask::rwx,other::rwx
    //Since fuse doesn't have a way to expose mask to the user, we only are concerned about
    // user, group and other.
    std::string result = "user::";

    result.push_back(mode & (1 << 8) ? 'r': '-');
    result.push_back(mode & (1 << 7) ? 'w' : '-');
    result.push_back(mode & (1 << 6) ? 'x' : '-');

    result += ",group::";
    result.push_back(mode & (1 << 5) ? 'r' : '-');
    result.push_back(mode & (1 << 4) ? 'w' : '-');
    result.push_back(mode & (1 << 3) ? 'x' : '-');

    // Push back the string with each of the mode segments
    result += ",other::";
    result.push_back(mode & (1 << 2) ? 'r' : '-');
    result.push_back(mode & (1 << 1) ? 'w' : '-');
    result.push_back(mode & 01 ? 'x' : '-');

    return result;
}

mode_t aclToMode(access_control acl)
{
    mode_t mode = 0;

    std::string permissions = acl.permissions;
    if(permissions.empty())
    {
        syslog(LOG_ERR, "Failure to convert permissions, empty permissions from service");
        return mode;
    }
    else if(permissions.size() != blobfuse_constants::acl_size)
    {
        syslog(LOG_ERR, "Failure: Unexpected amount of permissions from service : %s", permissions.c_str());
        return mode;
    }
    //try 
    {
        if (permissions[0] == 'r')
            mode |= 0400;
        if (permissions[1] == 'w')
            mode |= 0200;
        if (permissions[2] == 'x')
            mode |= 0100;
        if (permissions[3] == 'r')
            mode |= 0040;
        if (permissions[4] == 'w')
            mode |= 0020;
        if (permissions[5] == 'x')
            mode |= 0010;
        if (permissions[6] == 'r')
            mode |= 0004;
        if (permissions[7] == 'w')
            mode |= 0002;
        if (permissions[8] == 'x')
            mode |= 0001;
    }
    /*catch(std::exception err)
    {
        syslog(LOG_ERR, "Failure parsing permissions from service");
        return 0;
    }*/
    return mode;
}
