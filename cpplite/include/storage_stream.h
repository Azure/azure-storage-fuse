#pragma once

#include <iostream>
#include <memory>
#include <sstream>

#include "storage_EXPORTS.h"

namespace azure {  namespace storage_lite {

    class storage_istream
    {
    public:
        storage_istream() {}

        storage_istream(std::istream& stream) : m_initial(stream.tellg()), m_stream(&stream, [](std::istream*) { /* null deleter */ }) {}

        storage_istream(std::shared_ptr<std::istream> stream) : m_initial(stream->tellg()), m_stream(std::move(stream)) {}

        void reset()
        {
            if (valid())
            {
                m_stream->seekg(m_initial);
            }
        }

        std::istream& istream()
        {
            return *m_stream;
        }

        bool valid() const
        {
            return m_stream != nullptr;
        }

    private:
        std::istream::off_type m_initial;
        std::shared_ptr<std::istream> m_stream;
    };

    class storage_ostream
    {
    public:
        storage_ostream() {}

        storage_ostream(std::ostream& stream) : m_initial(stream.tellp()), m_stream(&stream, [](std::ostream*) { /* null deleter */ }) {}

        storage_ostream(std::shared_ptr<std::ostream> stream) : m_initial(stream->tellp()), m_stream(std::move(stream)) {}

        std::ostream& ostream()
        {
            return *m_stream;
        }

        void reset()
        {
            if (valid())
            {
                m_stream->seekp(m_initial);
            }
        }

        bool valid() const
        {
            return m_stream != nullptr;
        }

    private:
        std::ostream::off_type m_initial;
        std::shared_ptr<std::ostream> m_stream;
    };

    class storage_iostream : public storage_istream, public storage_ostream
    {
    public:
        static storage_iostream create_storage_stream()
        {
            return storage_iostream(std::make_shared<std::stringstream>());
        }

        storage_iostream() {}

        storage_iostream(std::iostream& stream) : storage_istream(stream), storage_ostream(stream) {}

    private:
        storage_iostream(std::shared_ptr<std::iostream> s) : storage_istream(*s), storage_ostream(*s), m_stream(s) {}

        std::shared_ptr<std::iostream> m_stream;
    };

}}  // azure::storage_lite
