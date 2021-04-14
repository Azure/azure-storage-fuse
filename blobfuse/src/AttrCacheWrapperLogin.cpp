#include <blobfuse.h>
#include <boost/filesystem.hpp>
#include <string>
#include <signal.h>
#include <mntent.h>
#include <sys/types.h>
#include <dirent.h>
#include <storage_credential.h>
#include <BlobfuseConstants.h>
#include <AttrCacheWrapper.h>

using namespace azure::storage_lite;

namespace azure { namespace storage_lite {

    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_accountkey(
        const std::string &account_name,
        const std::string &account_key,
        const unsigned int concurrency,
        bool use_https,
        const std::string &blob_endpoint)
    {
        /* set a default concurrency value. */
        unsigned int concurrency_limit = 40;
        if(concurrency != 0)
        {
            concurrency_limit = concurrency;
        }
        std::string accountName(account_name);
        std::string accountKey(account_key);
        try
        {
            std::shared_ptr<storage_credential> cred;
            if (account_key.length() > 0)
            {
                cred = std::make_shared<shared_key_credential>(accountName, accountKey);
            }
            else
            {
                syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
                return std::make_shared<blob_client_wrapper>(false);
            }
            std::shared_ptr<storage_account> account = std::make_shared<storage_account>(accountName, cred, use_https, blob_endpoint);
            std::shared_ptr<blob_client> blobClient= std::make_shared<azure::storage_lite::blob_client>(account, concurrency_limit, config_options.caCertPath);
            errno = 0;
            return std::make_shared<blob_client_wrapper>(blobClient);
        }
        catch(const std::exception &ex)
        {
            syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
            errno = blobfuse_constants::unknown_error;
            return std::make_shared<blob_client_wrapper>(false);
        }
    }


    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_sastoken(
        const std::string &account_name,
        const std::string &sas_token,
        const unsigned int concurrency,
        bool use_https,
        const std::string &blob_endpoint)
    {
        /* set a default concurrency value. */
        unsigned int concurrency_limit = 40;
        if(concurrency != 0)
        {
            concurrency_limit = concurrency;
        }
        std::string accountName(account_name);
        std::string sasToken(sas_token);

        try
        {
            std::shared_ptr<storage_credential> cred;
            if(sas_token.length() > 0)
            {
                cred = std::make_shared<shared_access_signature_credential>(sas_token);
            }
            else
            {
                syslog(LOG_ERR, "Empty account key. Failed to create blob client.");
                return std::make_shared<blob_client_wrapper>(false);
            }
            std::shared_ptr<storage_account> account = std::make_shared<storage_account>(accountName, cred, use_https, blob_endpoint);
            std::shared_ptr<blob_client> blobClient= std::make_shared<azure::storage_lite::blob_client>(account, concurrency_limit, config_options.caCertPath);
            errno = 0;
            return std::make_shared<blob_client_wrapper>(blobClient);
        }
        catch(const std::exception &ex)
        {
            syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
            errno = blobfuse_constants::unknown_error;
            return std::make_shared<blob_client_wrapper>(false);
        }
    }

    std::shared_ptr<blob_client_wrapper> blob_client_wrapper_init_oauth(
        const std::string &account_name,
        const unsigned int concurrency,
        const std::string &blob_endpoint)
    {
        /* set a default concurrency value. */
        unsigned int concurrency_limit = 40;
        if(concurrency != 0)
        {
            concurrency_limit = concurrency;
        }
        std::string accountName(account_name);

        try
        {
            std::shared_ptr<storage_credential> cred = std::make_shared<token_credential>("");
            std::shared_ptr<storage_account> account = std::make_shared<storage_account>(
                accountName,
                cred,
                true, //use_https must be true to use oauth
                blob_endpoint);
            std::shared_ptr<blob_client> blobClient =
                std::make_shared<azure::storage_lite::blob_client>(account, concurrency_limit, config_options.caCertPath);
            errno = 0;
            return std::make_shared<blob_client_wrapper>(blobClient);
        }
        catch(const std::exception &ex)
        {
            syslog(LOG_ERR, "Failed to create blob client.  ex.what() = %s.", ex.what());
            errno = blobfuse_constants::unknown_error;
            return std::make_shared<blob_client_wrapper>(false);
        }
    }

} }
