#pragma once

#if defined (_MSC_VER)
#define AZURE_STORAGE_API __declspec(dllexport)
#else /* defined (_MSC_VER) */
#define AZURE_STORAGE_API
#endif
