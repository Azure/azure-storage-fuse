#pragma once

#include <streambuf>
#include <istream>
#include <ostream>

#pragma push_macro("_SCL_SECURE_NO_WARNINGS")
#define _SCL_SECURE_NO_WARNINGS

namespace azure { namespace storage_lite {

    class memory_streambuf : public std::streambuf
    {
    public:
        memory_streambuf(char* data, uint64_t size) : m_data(data), m_cur(data), m_end(data + size) {}

    protected:
        std::streamsize showmanyc() override
        {
            return m_end - m_cur;
        }

        int_type underflow() override
        {
            return m_cur < m_end ? traits_type::to_int_type(*m_cur) : traits_type::eof();
        }

        int_type uflow() override
        {
            return m_cur < m_end ? traits_type::to_int_type(*m_cur++) : traits_type::eof();
        }

        int_type overflow(int_type ch) override
        {
            if (traits_type::eq_int_type(ch, traits_type::eof()))
            {
                return traits_type::not_eof(ch);
            }
            if (m_cur == m_end)
            {
                return traits_type::eof();
            }
            *m_cur++ = traits_type::to_char_type(ch);
            return ch;
        }

        std::streamsize xsgetn(char_type* s, std::streamsize count) override
        {
            std::streamsize read_count = std::min(count, showmanyc());
            std::copy(m_cur, m_cur + read_count, s);
            m_cur += read_count;
            return read_count;
        }

        std::streamsize xsputn(const char_type* s, std::streamsize count) override
        {
            std::streamsize write_count = std::min(count, showmanyc());
            std::copy(s, s + write_count, m_cur);
            m_cur += write_count;
            return write_count;
        }

        pos_type seekoff(off_type off, std::ios_base::seekdir dir, std::ios_base::openmode which) override
        {
            if (which & (std::ios_base::in | std::ios_base::out)) {
                if (dir == std::ios_base::cur)
                {
                    m_cur += off;
                }
                else if (dir == std::ios_base::end)
                {
                    m_cur = m_end + off;
                }
                else if (dir == std::ios_base::beg)
                {
                    m_cur = m_data + off;
                }
                m_cur = std::min(m_cur, m_end);
                m_cur = std::max(m_data, m_cur);
            }
            return m_cur - m_data;
        }

        pos_type seekpos(pos_type pos, std::ios_base::openmode which) override
        {
            return seekoff(pos, std::ios_base::beg, which);
        }

    private:
        char_type* m_data;
        char_type* m_cur;
        char_type* m_end;
    };

    class imstream : public std::istream
    {
    public:
        imstream(const char* data, uint64_t size) : std::istream(&m_streambuf), m_streambuf(const_cast<char*>(data), size) {}
    private:
        memory_streambuf m_streambuf;
    };

    class omstream : public std::ostream
    {
    public:
        omstream(char* data, uint64_t size) : std::ostream(&m_streambuf), m_streambuf(data, size) {}
    private:
        memory_streambuf m_streambuf;
    };

    class mstream : public std::iostream
    {
    public:
        mstream(char* data, uint64_t size) : std::iostream(&m_streambuf), m_streambuf(data, size) {}
    private:
        memory_streambuf m_streambuf;
    };

}}  // azure::storage_lite

#pragma pop_macro("_SCL_SECURE_NO_WARNINGS")
