#pragma once

#if defined(_WIN32) && defined(_WINDLL)
    #ifdef azure_storage_adls_EXPORTS
    #define AZURE_STORAGE_ADLS_API __declspec(dllexport)
    #else
    #define AZURE_STORAGE_ADLS_API __declspec(dllimport)
    #endif
#else /* defined(_WIN32) && defined(_WINDLL) */
    #define AZURE_STORAGE_ADLS_API
#endif

#include "storage_outcome.h"
#include "blob/blob_client.h"
#include "set_access_control_request.h"
#include "list_paths_request.h"

namespace azure { namespace storage_adls {
    using storage_account = azure::storage_lite::storage_account;
    using executor_context = azure::storage_lite::executor_context;
    using storage_exception = azure::storage_lite::storage_exception;

    struct list_filesystems_item
    {
        std::string name;
    };

    struct list_filesystems_result
    {
        std::vector<list_filesystems_item> filesystems;
        std::string continuation_token;
    };

    /// <summary>
    /// Provides a client-side representation of ADLS Gen2 service on Microsoft Azure. This client is used to configure and execute requests against the service.
    /// </summary>
    /// <remarks>The service client encapsulates the base URI for the service. If the service client will be used for authenticated access, it also encapsulates the credentials for accessing the storage account.</remarks>
    class adls_client final
    {
    public:
        /// <summary>
        /// Initializes a new instance of the <see cref="azure::storage_adls::adls_client" /> class.
        /// <param name="account">An existing <see cref="azure::storage_alds::storage_account" /> object.</param>
        /// <param name="max_concurrency">An int value indicates the maximum concurrency expected during executing requests against the service.</param>
        /// <param name="exception_enabled">Whether to use exception or errno for error handling.</param>
        AZURE_STORAGE_ADLS_API adls_client(std::shared_ptr<storage_account> account, int max_concurrency, bool exception_enabled = true);

        /// <summary>
        /// Creates a filesystem. If the filesystem already exists, the operation fails.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        AZURE_STORAGE_ADLS_API void create_filesystem(const std::string& filesystem);

        /// <summary>
        /// Marks the filesystem for deletion.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        AZURE_STORAGE_ADLS_API void delete_filesystem(const std::string& filesystem);

        /// <summary>
        /// Returns whether a filesystem already exists.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <returns><c>true</c> if the filesystem exists, <c>false</c> otherwise.</returns>
        /// <remarks>Authentication failure may also lead this function to return false.</remarks>
        AZURE_STORAGE_ADLS_API bool filesystem_exists(const std::string& filesystem);

        /// <summary>
        /// Sets properties for the filesystem.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="properties">Key-value pairs of properties</param>
        AZURE_STORAGE_ADLS_API void set_filesystem_properties(const std::string& filesystem, const std::vector<std::pair<std::string, std::string>>& properties);

        /// <summary>
        /// Gets properties for the filesystem.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <returns>Key-value pairs of properties.</returns>
        AZURE_STORAGE_ADLS_API std::vector<std::pair<std::string, std::string>> get_filesystem_properties(const std::string& filesystem);

        /// <summary>
        /// Lists filesystems.
        /// </summary>
        /// <param name="prefix">Filter results to filesystems within the specified prefix.</param>
        /// <param name="continuation_token">
        /// The number of filesystems returned with each invocation is limited. If the number of filesystems to be returned
        /// exceeds this limit, a continuation token is returned in the response
        /// When a continuation token is returned in the response, it must be specified in a subsequent invocation of the
        /// list operation to continue listing the filesystems.
        /// </param>
        /// <param name="max_results">An optional value that specifies the maximum number of items to return.</param>
        /// <returns>A <see cref="azure::storage_adls::list_filesystems_result"> object which contains filesystems list and continuation token.</returns>
        AZURE_STORAGE_ADLS_API list_filesystems_result list_filesystems_segmented(const std::string& prefix, const std::string& continuation_token = std::string(), const int max_results = 0);

        /// <summary>
        /// Creates a directory in the filesystem. If the directory already exists, it's overwritten.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        AZURE_STORAGE_ADLS_API void create_directory(const std::string& filesystem, const std::string& directory);

        /// <summary>
        /// Deletes the directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        AZURE_STORAGE_ADLS_API void delete_directory(const std::string& filesystem, const std::string& directory);

        /// <summary>
        /// Returns whether a directory already exists.
        /// </summary>
        /// <param name="filesystem">The directory name.</param>
        /// <param name="directory">The directory path.</param>
        /// <returns><c>true</c> if the directory exists, <c>false</c> otherwise.</returns>
        /// <remarks>Authentication failure may also lead this function to return false.</remarks>
        AZURE_STORAGE_ADLS_API bool directory_exists(const std::string& filesystem, const std::string& directory);

        /// <summary>
        /// Moves a directory into another directory inside the filesystem.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="source_path">The source directory path.</param>
        /// <param name="destination_path">The destination directory path.</param>
        AZURE_STORAGE_ADLS_API void move_directory(const std::string& filesystem, const std::string& source_path, const std::string& destination_path);

        /// <summary>
        /// Moves a directory into another directory, the destination can be outside the source filesystem.
        /// </summary>
        /// <param name="source_filesystem">The source filesystem name.</param>
        /// <param name="source_path">The source directory path.</param>
        /// <param name="destination_filesystem">The destination filesystem name.</param>
        /// <param name="destination_path">The destination directory.</param>
        AZURE_STORAGE_ADLS_API void move_directory(const std::string& source_filesystem, const std::string& source_path, const std::string& destination_filesystem, const std::string& destination_path);

        /// <summary>
        /// Sets properties for the directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        /// <param name="properties">Key-value pairs of properties</param>
        AZURE_STORAGE_ADLS_API void set_directory_properties(const std::string& filesystem, const std::string& directory, const std::vector<std::pair<std::string, std::string>>& properties);

        /// <summary>
        /// Gets properties for the directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        /// <returns>Key-value pairs of properties.</returns>
        AZURE_STORAGE_ADLS_API std::vector<std::pair<std::string, std::string>> get_directory_properties(const std::string& filesystem, const std::string& directory);

        /// <summary>
        /// Sets access control for a directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        /// <param name="acl">A <see cref="azure::azure_adls::access_control" /> object that represents POSIX access control.</param>
        AZURE_STORAGE_ADLS_API void set_directory_access_control(const std::string& filesystem, const std::string& directory, const access_control& acl);

        /// <summary>
        /// Gets access control for a directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">The directory path.</param>
        /// <returns>A <see cref="azure::azure_adls::access_control" /> object that represents POSIX access control.</returns>
        AZURE_STORAGE_ADLS_API access_control get_directory_access_control(const std::string& filesystem, const std::string& directory);

        /// <summary>
        /// Lists filesystem paths and their properties.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="directory">Filter results to paths within the specified directory.</param>
        /// <param name="recursive">Recursively lists subdirectories encountered.</param>
        /// <param name="continuation_token">
        /// The number of paths returned with each invocation is limited. If the number of paths to be returned
        /// exceeds this limit, a continuation token is returned in the response.
        /// When a continuation token is returned in the response, it must be specified in a subsequent invocation of the
        /// list operation to continue listing the directory.
        /// </param>
        /// <param name="max_results">An optional value that specifies the maximum number of items to return.</param>
        /// <returns>A <see cref="azure::storage_adls::list_paths_result" /> object which contains paths list and continuation token.</returns>
        AZURE_STORAGE_ADLS_API list_paths_result list_paths_segmented(const std::string& filesystem, const std::string& directory, bool recursive = false, const std::string& continuation_token = std::string(), const int max_results = 0);

        /// <summary>
        /// Creates an empty file in the filesystem. If the file already exists, it'll be overwritten.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        AZURE_STORAGE_ADLS_API void create_file(const std::string& filename, const std::string& file);

        /// <summary>
        /// Uploads data to be appended to a file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="offset">The position where the data is to be appended.</param>
        /// <param name="in_stream">The source stream.</param>
        /// <param name="stream_len">Length of the stream.</param>
        AZURE_STORAGE_ADLS_API void append_data_from_stream(const std::string& filesystem, const std::string& file, uint64_t offset, std::istream& in_stream, uint64_t stream_len = 0);

        /// <summary>
        /// Flushes previously uploaded data to a file. To flush, the previously uploaded data must be contiguous.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="offset">The position must be equal to the length of the file after all data has been written. </param>
        AZURE_STORAGE_ADLS_API void flush_data(const std::string& filesystem, const std::string& file, uint64_t offset);

        /// <summary>
        /// Uploads the contents of a stream to a file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="in_stream">The source stream.</param>
        /// <param name="properties">Key-value pairs of properties.</param>
        AZURE_STORAGE_ADLS_API void upload_file_from_stream(const std::string& filesystem, const std::string& file, std::istream& in_stream, const std::vector<std::pair<std::string, std::string>>& properties = std::vector<std::pair<std::string, std::string>>());

        /// <summary>
        /// Downloads the contents of a file to a stream.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="out_stream">The target stream.</param>
        AZURE_STORAGE_ADLS_API void download_file_to_stream(const std::string& filesystem, const std::string& file, std::ostream& out_stream);

        /// <summary>
        /// Downloads the contents of a file to a stream.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="offset">The offset where to begin downloading the file, in bytes.</param>
        /// <param name="size">The size of data to download, in bytes. Specify 0 if you want to download till end.</param>
        /// <param name="out_stream">The target stream.</param>
        AZURE_STORAGE_ADLS_API void download_file_to_stream(const std::string& filesystem, const std::string& file, uint64_t offset, uint64_t size, std::ostream& out_stream);

        /// <summary>
        /// Deletes the file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        AZURE_STORAGE_ADLS_API void delete_file(const std::string& filesystem, const std::string& file);

        /// <summary>
        /// Returns whether a file already exists.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <returns><c>true</c> if the file exists, <c>false</c> otherwise.</returns>
        /// <remarks>Authentication failure may also lead this function to return false.</remarks>
        AZURE_STORAGE_ADLS_API bool file_exists(const std::string& filesystem, const std::string& file);

        /// <summary>
        /// Moves a file into another place within the filesystem.
        /// If destination is a file, then it's overwritten.
        /// If destination is a directory, the source file is moved into the directory.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="source_path">The source file path.</param>
        /// <param name="destination_path">The destination file or directory path.</param>
        AZURE_STORAGE_ADLS_API void move_file(const std::string& filesystem, const std::string& source_path, const std::string& destination_path);

        /// <summary>
        /// Moves a file into another place, the destination can be outside the source filesystem.
        /// If destination is a file, then it's overwritten.
        /// If destination is a directory, the source file is moved into the directory.
        /// </summary>
        /// <param name="source_filesystem">The source filesystem name.</param>
        /// <param name="source_path">The source file path.</param>
        /// <param name="destination_filesystem">The destination filesystem.</param>
        /// <param name="destination_path">The destination file or directory path.</param>
        AZURE_STORAGE_ADLS_API void move_file(const std::string& source_filesystem, const std::string& source_path, const std::string& destination_filesystem, const std::string& destination_path);

        /// <summary>
        /// Sets properties for the file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="properties">Key-value pairs of properties</param>
        AZURE_STORAGE_ADLS_API void set_file_properties(const std::string& filesystem, const std::string& file, const std::vector<std::pair<std::string, std::string>>& properties);

        /// <summary>
        /// Gets properties for the file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <returns>Key-value pairs of properties.</returns>
        AZURE_STORAGE_ADLS_API std::vector<std::pair<std::string, std::string>> get_file_properties(const std::string& filesystem, const std::string& file);

        /// <summary>
        /// Sets access control for a file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <param name="acl">A <see cref="azure::azure_adls::access_control" /> object that represents POSIX access control.</param>
        AZURE_STORAGE_ADLS_API void set_file_access_control(const std::string& filesystem, const std::string& file, const access_control& acl);

        /// <summary>
        /// Gets access control for a file.
        /// </summary>
        /// <param name="filesystem">The filesystem name.</param>
        /// <param name="file">The file path.</param>
        /// <returns>A <see cref="azure::azure_adls::access_control" /> object that represents POSIX access control.</returns>
        AZURE_STORAGE_ADLS_API access_control get_file_access_control(const std::string& filesystem, const std::string& file);

        /// <summary>
        /// Returns whether exception is enabled for this <see cref="azure::storage_adls::adls_client" />.
        /// </summary>
        /// <returns><c>true</c> if exception is enabled, <c>false</c> otherwise.</returns>
        bool exception_enabled() const
        {
            return m_exception_enabled;
        }
    private:
        template<class RET, class FUNC>
        RET blob_client_adaptor(FUNC func);

        bool success() const
        {
            return !(!m_exception_enabled && errno != 0);
        }

    private:
        std::shared_ptr<azure::storage_lite::blob_client> m_blob_client;
        std::shared_ptr<storage_account> m_account;
        std::shared_ptr<executor_context> m_context;

        const bool m_exception_enabled;
    };

}}  // azure::storage_adls
