/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2021 Microsoft Corporation. All rights reserved.
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

#ifndef __WINFSP_H__
#define __WINFSP_H__

#define FSP_FSCTL_PRODUCT_NAME          "WinFsp"

#if defined(_WIN64)
#define FSP_DLLNAME                     "winfsp-x64.dll"
#else
#define FSP_DLLNAME                     "winfsp-x86.dll"
#endif

#define FSP_DLLPATH                     "bin\\" FSP_DLLNAME
static void*                            winfsp_handle;

static inline
int winfsp_init()
{
    WINADVAPI
    LSTATUS
    APIENTRY
    RegGetValueW(
        HKEY hkey,
        LPCWSTR lpSubKey,
        LPCWSTR lpValue,
        DWORD dwFlags,
        LPDWORD pdwType,
        PVOID pvData,
        LPDWORD pcbData);

    WCHAR PathBuf[MAX_PATH];
    DWORD Size;
    HKEY RegKey;
    LONG Result;
    HMODULE Module;

    wprintf(L"Trying to find WinFsp Dll \n");

    if (NULL != winfsp_handle)
        winfsp_handle = NULL;

    winfsp_handle = LoadLibraryW(L"" FSP_DLLNAME);
    if (NULL == winfsp_handle)
    {
        Result = RegOpenKeyExW(HKEY_LOCAL_MACHINE, L"Software\\" FSP_FSCTL_PRODUCT_NAME,
            0, KEY_READ | KEY_WOW64_32KEY, &RegKey);
        if (ERROR_SUCCESS == Result)
        {
            Size = sizeof PathBuf - sizeof L"" FSP_DLLPATH + sizeof(WCHAR);
            Result = RegGetValueW(RegKey, 0, L"InstallDir",
                RRF_RT_REG_SZ, 0, PathBuf, &Size);
            RegCloseKey(RegKey);
        }
        if (ERROR_SUCCESS != Result)
            return -2;

        RtlCopyMemory(PathBuf + (Size / sizeof(WCHAR) - 1), L"" FSP_DLLPATH, sizeof L"" FSP_DLLPATH);
        winfsp_handle = LoadLibraryW(PathBuf);
    }

	if (!winfsp_handle)
	    return -9;

    wprintf(L"Dll Loaded\n");
    return 0;
}

static int winfsp_statvfs(const char *path, struct fuse_statvfs *stbuf)
{
    char root[PATH_MAX];
    DWORD
        VolumeSerialNumber,
        MaxComponentLength,
        SectorsPerCluster,
        BytesPerSector,
        NumberOfFreeClusters,
        TotalNumberOfClusters;

    if (!GetVolumePathNameA(path, root, PATH_MAX) ||
        !GetVolumeInformationA(root, 0, 0, &VolumeSerialNumber, &MaxComponentLength, 0, 0, 0) ||
        !GetDiskFreeSpaceA(root, &SectorsPerCluster, &BytesPerSector,
            &NumberOfFreeClusters, &TotalNumberOfClusters))
    {
        return -99;
    }

    memset(stbuf, 0, sizeof *stbuf);
    stbuf->f_bsize = SectorsPerCluster * BytesPerSector;
    stbuf->f_frsize = SectorsPerCluster * BytesPerSector;
    stbuf->f_blocks = TotalNumberOfClusters;
    stbuf->f_bfree = NumberOfFreeClusters;
    stbuf->f_bavail = TotalNumberOfClusters;
    stbuf->f_fsid = VolumeSerialNumber;
    stbuf->f_namemax = MaxComponentLength;

    return 0;
}

#endif //__WINFSP_H__