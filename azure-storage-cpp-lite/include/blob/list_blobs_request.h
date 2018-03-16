#pragma once

#include "list_blobs_request_base.h"

namespace microsoft_azure {
namespace storage {

class list_blobs_request : public list_blobs_request_base {
public:
    list_blobs_request(const std::string &container, const std::string &prefix)
        : m_container(container),
          m_prefix(prefix) {}

    std::string container() const override {
        return m_container;
    }

    std::string prefix() const override {
        return m_prefix;
    }

    std::string marker() const override {
        return m_marker;
    }

    int maxresults() const override {
        return m_maxresults;
    }

    list_blobs_request &set_marker(const std::string &marker) {
        m_marker = marker;
        return *this;
    }

    list_blobs_request &set_maxresults(int maxresults) {
        m_maxresults = maxresults;
        return *this;
    }

private:
    std::string m_container;
    std::string m_prefix;
    std::string m_marker;
    int m_maxresults;
};

class list_blobs_hierarchical_request : public list_blobs_hierarchical_request_base {
public:
    list_blobs_hierarchical_request(const std::string &container, const std::string &delimiter, const std::string &continuation_token, const std::string &prefix)
        : m_container(container),
          m_prefix(prefix),
          m_marker(continuation_token),
          m_delimiter(delimiter),
          m_maxresults(0) {}

    std::string container() const override {
        return m_container;
    }

    std::string prefix() const override {
        return m_prefix;
    }

    std::string marker() const override {
        return m_marker;
    }

    std::string delimiter() const override {
        return m_delimiter;
    }

    int maxresults() const override {
        return m_maxresults;
    }

    list_blobs_request_base::include includes() const override{ 
        return m_includes; 
    }


    list_blobs_hierarchical_request &set_marker(const std::string &marker) {
        m_marker = marker;
        return *this;
    }

    list_blobs_hierarchical_request &set_maxresults(int maxresults) {
        m_maxresults = maxresults;
        return *this;
    }

    list_blobs_hierarchical_request &set_includes(list_blobs_request_base::include includes) {
        m_includes = includes;
        return *this;
    }

private:
    std::string m_container;
    std::string m_prefix;
    std::string m_marker;
    std::string m_delimiter;
    int m_maxresults;
    list_blobs_request_base::include m_includes;
};
}
}
