#include "FileLockMap.h"

file_lock_map* file_lock_map::get_instance()
{
    if(nullptr == s_instance.get())
    {
        std::lock_guard<std::mutex> lock(s_mutex);
        if(nullptr == s_instance.get())
        {
            s_instance.reset(new file_lock_map());
        }
    }
    return s_instance.get();
}

std::shared_ptr<std::mutex> file_lock_map::get_mutex(const std::string& path)
{
    std::lock_guard<std::mutex> lock(m_mutex);
    auto iter = m_lock_map.find(path);
    if(iter == m_lock_map.end())
    {
        auto file_mutex = std::make_shared<std::mutex>();
        m_lock_map[path] = file_mutex;
        return file_mutex;
    }
    else
    {
        return iter->second;
    }
}

std::shared_ptr<std::mutex> file_lock_map::get_delay_mutex(const std::string& path)
{
    std::lock_guard<std::mutex> lock(d_mutex);
    auto iter = m_delay_map.find(path);
    if(iter == m_delay_map.end())
    {
        auto file_mutex = std::make_shared<std::mutex>();
        m_delay_map[path] = file_mutex;
        return file_mutex;
    }
    else
    {
        return iter->second;
    }
}