#ifndef DATALAKEBFSCLIENT_H
#define DATALAKEBFSCLIENT_H

#include <BlobfuseGlobals.h>
#include <BlockBlobBfsClient.h>
#include <DfsProperties.h>

using namespace azure::storage_lite;

class DataLakeBfsClient : public BlockBlobBfsClient
{
public:
    DataLakeBfsClient(configParams config_options) :
    BlockBlobBfsClient(config_options),
    m_adls_client(NULL)
    {}

    bool isADLS() { return true; }
    
    ///<summary>
    /// Authenticates the storage account and container
    ///</summary>
    ///<returns>bool: if we authenticate to the storage account and container successfully</returns>
    bool AuthenticateStorage() override;
    ///<summary>
    /// Creates a Directory
    ///</summary>
    ///<returns>none</returns>
    bool CreateDirectory(const std::string directoryPath) override;
    ///<summary>
    /// Deletes a Directory
    ///</summary>
    ///<returns>none</returns>
    bool DeleteDirectory(const std::string directoryPath) override;

     ///<summary>
    /// Does the ADLS Directory or blob exist
    ///</summary>
    ///<returns>none</returns>
    int Exists(const std::string directoryPath) override;
    
    ///<summary>
    /// Helper function - Checks if the "directory" blob is empty
    ///</summary>
    D_RETURN_CODE IsDirectoryEmpty(std::string path) override;
    ///<summary>
    /// Renames a DataLake file
    ///</summary>
    ///<returns>none</returns>
    std::vector<std::string> Rename(std::string sourcePath, std::string destinationPath) override;
    std::vector<std::string> Rename(const std::string sourcePath,const  std::string destinationPath, bool isDir) override;
    
    ///<summary>
    /// Lists
    ///</summary>
    ///<returns>none</returns>
    void List(std::string continuation, std::string prefix, std::string delimiter, list_segmented_response &resp, int max_results = 10000) override;
    ///<summary>
    /// Updates the UNIX-style file mode on a path.
    ///</summary>
    int ChangeMode(const char* path, mode_t mode) override;
    ///<summary>
    /// Gets the properties of a path
    ///</summary>
    ///<returns>BfsFileProperty object which contains the property details of the file</returns>
    BfsFileProperty GetProperties(std::string pathName, bool type_known = false) override;
    void GetExtraProperties(const std::string pathName, BfsFileProperty &prop) override;
    
    virtual int UpdateBlobProperty(std::string pathStr, std::string key, std::string value, METADATA *metadata = NULL);

private:
    ///<summary>
    /// Helper function - Authenticates with an account key
    ///</summary>
    std::shared_ptr<adls_client_ext> authenticate_adls_accountkey();
    ///<summary>
    /// Helper function - Authenticates with an account sas
    ///</summary>
    std::shared_ptr<adls_client_ext> authenticate_adls_sas();
    ///<summary>
    /// Helper function - Authenticates with msi
    ///</summary>
    std::shared_ptr<adls_client_ext> authenticate_adls_msi();
    ///<summary>
    /// Helper function - Authenticates with spn
    ///</summary>
    std::shared_ptr<adls_client_ext> authenticate_adls_spn();
    ///<summary>
    /// Helper function - Renames cached files
    ///</summary>
    ///<returns>Error value</return>
    long int rename_cached_file(std::string src, std::string dest);
    ///<summary>
    /// ADLS Client to make dfs storage calls
    ///</summary>
    std::shared_ptr<adls_client_ext> m_adls_client;
};

#endif //DATALAKEBFSCLIENT_H