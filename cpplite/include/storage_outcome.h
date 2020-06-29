#pragma once

#include <string>
#include <exception>

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {

    class storage_error
    {
    public:
        std::string code;
        std::string code_name;
        std::string message;
    };

    struct storage_exception : public std::exception
    {
        storage_exception(int code, std::string code_name, std::string message) : code(code), code_name(std::move(code_name)), message(std::move(message)) {}

        int code;
        std::string code_name;
        std::string message;
    };

    template<typename RESPONSE_TYPE>
    class storage_outcome
    {
    public:
        storage_outcome()
            : m_success(false) {}

        storage_outcome(RESPONSE_TYPE response)
            : m_success(true),
            m_response(std::move(response)) {}

        storage_outcome(storage_error error)
            : m_success(false),
            m_error(std::move(error)) {}

        bool success() const
        {
            return m_success;
        }

        const storage_error &error() const
        {
            return m_error;
        }

        const RESPONSE_TYPE &response() const
        {
            return m_response;
        }

    private:
        bool m_success;
        storage_error m_error;
        RESPONSE_TYPE m_response;
    };

    template<>
    class storage_outcome<void>
    {
    public:
        storage_outcome()
            : m_success(true) {}

        storage_outcome(storage_error error)
            : m_success(false),
            m_error(std::move(error)) {}

        bool success() const
        {
            return m_success;
        }

        const storage_error &error() const
        {
            return m_error;
        }

        void response() const
        {
        }

    private:
        bool m_success;
        storage_error m_error;
    };

}}  // azure::storage_lite
