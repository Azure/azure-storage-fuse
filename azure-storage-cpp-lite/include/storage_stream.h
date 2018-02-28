#pragma once

#include <iostream>
#include <memory>
#include <sstream>

#include "storage_EXPORTS.h"

namespace microsoft_azure {
    namespace storage {

        class storage_istream_helper {
        public:
            storage_istream_helper(std::istream &stream)
                : m_stream(stream) {}

            std::istream &istream() {
                return m_stream;
            }

        private:
            std::istream &m_stream;
        };

        class storage_istream {
        public:
            storage_istream() {}

            storage_istream(std::istream &stream) {
                m_helper = std::make_shared<storage_istream_helper>(stream);
            }

            storage_istream(std::shared_ptr<std::istream> stream) {
                m_stream = stream;
            }

            void reset() {
               if (!valid()) {
                  return;
               }
               istream().seekg(0);
            }

            std::istream &istream() {
                if (m_helper) {
                    return m_helper->istream();
                }
                else {
                    return *m_stream;
                }
            }

            bool valid() const {
                return m_helper != nullptr || m_stream != nullptr;
            }

        private:
            std::shared_ptr<storage_istream_helper> m_helper;
            std::shared_ptr<std::istream> m_stream;
        };

        class storage_ostream_helper {
        public:
            storage_ostream_helper(std::ostream &stream)
                : m_stream(stream) {}

            std::ostream &ostream() {
                return m_stream;
            }

        private:
            std::ostream &m_stream;
        };

        class storage_ostream {
        public:
            storage_ostream() {}

            storage_ostream(std::ostream &stream) {
                m_initial = stream.tellp();
                m_helper = std::make_shared<storage_ostream_helper>(stream);
            }

            std::ostream &ostream() {
                return m_helper->ostream();
            }

            void reset() {
                if (!valid()) {
                    return;
                }
                ostream().seekp(m_initial);
            }

            bool valid() const {
                return m_helper != nullptr;
            }

        private:
            std::ostream::off_type m_initial;
            std::shared_ptr<storage_ostream_helper> m_helper;
        };

        class storage_iostream : public storage_istream, public storage_ostream {
        public:
            static storage_iostream create_storage_stream() {
                return storage_iostream(std::make_shared<std::stringstream>());
            }

            static storage_iostream create_storage_stream(const std::string &str) {
                return storage_iostream(std::make_shared<std::stringstream>(str));
            }

            storage_iostream() {}

            storage_iostream(std::iostream &stream)
                : storage_istream(stream),
                storage_ostream(stream) {}

        private:
            storage_iostream(std::shared_ptr<std::iostream> s)
                : storage_istream(*s),
                storage_ostream(*s),
                m_stream(s) {}

            std::shared_ptr<std::iostream> m_stream;
        };

    }
}
