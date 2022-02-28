/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2022 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

#ifndef __EXTENSION_HANDLER_H__
#define __EXTENSION_HANDLER_H__

#include <stdio.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h> 
#include <errno.h>
#include <dlfcn.h>

// Decide whether to add fuse2 or fuse3
#ifdef __FUSE2__
#include <fuse.h>
#else
#include <fuse3/fuse.h>
#endif

// -------------------------------------------------------------------------------------------------------------
// Extension loading and registeration methods
static  void        *extHandle = NULL;
typedef int         (*callback_exchanger)(struct fuse_operations *opts);
typedef const char* (*lib_validator)(const char* sign);
typedef int         (*lib_initializer)(const char* conf_file);


static callback_exchanger  ext_fuse_regsiter_func      = NULL;
static callback_exchanger  ext_storage_regsiter_func   = NULL;

static int load_library(char* extension_path) 
{
    // Load the configured library here
    extHandle = dlopen (extension_path, RTLD_LAZY);
    if (extHandle == NULL) {
        return 1;
    }

    // Get the function pointers from the lib and store them in given structure
    // Once we register these methods to libfuse, calls will directly land into extension
    lib_validator       ext_lib_validator_func      = NULL;
    lib_initializer     ext_lib_init_func           = NULL;

    ext_fuse_regsiter_func = (callback_exchanger)dlsym(extHandle, "register_fuse_callbacks");
    ext_storage_regsiter_func = (callback_exchanger)dlsym(extHandle, "register_storage_callbacks");
    ext_lib_validator_func = (lib_validator)dlsym(extHandle, "validate_signature");
    ext_lib_init_func = (lib_initializer)dlsym(extHandle, "init_extension");

    // Validate lib has legit functions exposed with this name
    if (ext_fuse_regsiter_func == NULL || ext_storage_regsiter_func == NULL || 
        ext_lib_validator_func == NULL || ext_lib_init_func == NULL) {
        return 2;
    }

    // Going for handshake with extension
    #ifdef __FUSE2__
    const char* my_call_sign = "ola-amigo!!";
    const char* lib_call_sign = "ola-amigo!!!";
    #else
    const char* my_call_sign = "ola-amigo-3!!";
    const char* lib_call_sign = "ola-amigo-3!!!";
    #endif

    const char* call_sign = ext_lib_validator_func(my_call_sign);
    if (strcmp(call_sign, lib_call_sign) != 0) {
        return 3;
    }

    if (0 != ext_lib_init_func("config.txt")) {
        return 4;
    }

    return 0;
}

static int unload_library() 
{
    if (extHandle) {
        dlclose(extHandle);
    }
    return 0;
}

static int get_extension_callbacks(fuse_operations_t* opt)
{
    if (!ext_fuse_regsiter_func) {
        return 1;
    }

    if (0 != ext_fuse_regsiter_func(opt)) {
        return 2;
    }

    return 0;
}

static int register_callback_to_extension(fuse_operations_t* opt)
{
    if (!ext_storage_regsiter_func) {
        return 1;
    }

    if (0 != ext_storage_regsiter_func(opt)) {
        return 2;
    }
    
    return 0;
}

// -------------------------------------------------------------------------------------------------------------
#endif //__EXTENSION_HANDLER_H__