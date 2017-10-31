#pragma once

#include <iterator>
#include <memory>
#include <map>
#include <string>

namespace microsoft_azure {
    namespace storage {

        enum class lease_status {
            locked,
            unlocked
        };

        enum class lease_state {
            available,
            leased,
            expired,
            breaking,
            broken
        };

        enum class lease_duration {
            none,
            infinite,
            fixed
        };

        enum class page_write {
            update,
            clear
        };

        enum class payload_format {
            json_fullmetadata,
            json_nometadata
        };

        class my_iterator_base : public std::iterator<std::input_iterator_tag, std::pair<std::string, std::string>> {
        public:
            virtual bool pass_end() const = 0;
            virtual value_type operator*() = 0;
            virtual void operator++() = 0;
        };

        class iterator_base : public std::iterator<std::input_iterator_tag, std::pair<std::string, std::string>> {
        public:
            iterator_base(std::shared_ptr<my_iterator_base> iterator) : m_iterator(iterator) {}

            bool operator!=(const iterator_base &other) const {
                if (m_iterator->pass_end() && other.m_iterator->pass_end())
                    return false;
                return true;
            }

            value_type operator*() {
                return **m_iterator;
            }

            iterator_base &operator++() {
                ++(*m_iterator);
                return *this;
            }

        private:
            std::shared_ptr<my_iterator_base> m_iterator;
        };

        class A {
        public:
            virtual iterator_base begin() = 0;
            virtual iterator_base end() = 0;
        };

        class my_iterator : public my_iterator_base {
        public:
            my_iterator(std::map<std::string, std::string>::iterator i, std::map<std::string, std::string>::iterator e) : it(i), end_it(e) {}

            void operator++() override {
                ++it;
            }

            value_type operator*() override {
                return std::make_pair(it->first, it->second);
            }

            bool pass_end() const override {
                return it == end_it;
            }

        private:
            std::map<std::string, std::string>::iterator it;
            std::map<std::string, std::string>::iterator end_it;
        };

        class B : public A {
        public:
            iterator_base begin() override {
                return iterator_base(std::make_shared<my_iterator>(m.begin(), m.end()));
            }

            iterator_base end() override {
                return iterator_base(std::make_shared<my_iterator>(m.end(), m.end()));
            }
            std::map<std::string, std::string> m;
        };

    }
}
