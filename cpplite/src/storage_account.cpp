#include "storage_account.h"

#include "constants.h"

namespace azure {  namespace storage_lite {

    std::shared_ptr<storage_account> storage_account::development_storage_account()
    {
        std::string account_name = "devstoreaccount1";
        std::string account_key = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==";
        std::shared_ptr<storage_credential>  cred = std::make_shared<shared_key_credential>(account_name, account_key);
        std::shared_ptr<storage_account> account = std::make_shared<storage_account>(account_name, cred, false, "127.0.0.1:10000/devstoreaccount1");
        return account;
    }

    // TODO: Clean up table queue and file services
    storage_account::storage_account(const std::string &account_name, std::shared_ptr<storage_credential> credential, bool use_https, const std::string &blob_endpoint)
        : m_credential(credential)
    {
        std::string scheme = use_https ? "https://" : "http://";

        if (blob_endpoint.empty())
        {
            std::string domain = scheme + account_name;

            m_blob_url.set_domain(domain + ".blob" + constants::default_endpoint_suffix);
            m_table_url.set_domain(domain + ".table" + constants::default_endpoint_suffix);
            m_queue_url.set_domain(domain + ".queue" + constants::default_endpoint_suffix);
            m_file_url.set_domain(domain + ".file" + constants::default_endpoint_suffix);
            m_adls_url.set_domain(domain + ".dfs" + constants::default_endpoint_suffix);
        }
        else
        {
            std::string endpoint = blob_endpoint;
            auto scheme_pos = endpoint.find("://");
            if (scheme_pos != std::string::npos)
            {
                endpoint = endpoint.substr(scheme_pos + 3);
            }

            auto slash_pos = endpoint.find('/');
            std::string host = endpoint.substr(0, slash_pos);

            auto path_start = endpoint.find_first_not_of('/', slash_pos);
            std::string path = path_start == std::string::npos ? "" : endpoint.substr(path_start);

            std::string domain = scheme + host;
            m_blob_url.set_domain(domain);
            m_table_url.set_domain(domain);
            m_queue_url.set_domain(domain);
            m_file_url.set_domain(domain);
            m_adls_url.set_domain(domain);

            if (!path.empty()) {
                m_blob_url.append_path(path);
                m_table_url.append_path(path);
                m_queue_url.append_path(path);
                m_file_url.append_path(path);
                m_adls_url.append_path(path);
            }
        }
    }

}}
