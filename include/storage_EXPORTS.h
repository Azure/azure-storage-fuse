#pragma once

#if defined(_WIN32) && defined(_WINDLL)
    #if defined(azure_storage_lite_EXPORTS)
        #define AZURE_STORAGE_API __declspec(dllexport)
    #else
        #define AZURE_STORAGE_API __declspec(dllimport)
    #endif
#else /* defined(_WIN32) && defined(_WINDLL) */
    #define AZURE_STORAGE_API
#endif
