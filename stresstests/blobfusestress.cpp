// Perf and stress test framework for blobfuse.
// There are many existing benchmarks for file systems, and those are useful, but it's also useful to have something
// purposefully designed to test and validate the expected behavior of blobfuse.
// For the purposes of this file, "stress" refers to validating that there is no data corruption or data loss at high scale, and 
// "perf" or "performance" refers to measuring throughput, latency, etc.
// We should run these tests for some static set of input parameters for every release.

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdexcept>
#include <string>
#include <iostream>
#include <sstream>
#include <iomanip>
#include <fstream>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <dirent.h>
#include <vector>
#include <sys/sendfile.h>
#include <errno.h>
#include <ftw.h>
#include <chrono>
#include <functional>
#include <algorithm>
#include <deque>
#include <mutex>
#include <condition_variable>
#include <thread>
#include <random>
#include <csignal>
#include <uuid/uuid.h>

void signalHandler( int signum ) {
   exit(signum);  
}

// Config
std::string perf_source_dir("/home/vikas/stress_test/src");  // Source directory for test data.  This is an SSD on my machine.  Should not be a blobfuse directory.  All contents will be wiped.
std::string perf_dest_dir_1("/home/vikas/blob_mnt/stress");  // blobfuse directory to copy to.  All contents will be wiped.
std::string perf_dest_dir_2("/home/vikas/stress_test/dst");  // Local destination directory.  This is an SSD on my machine.  Should not be a blobfuse directory.  All contents will be wiped.


// There isn't really a built-in C++11 threadpool, and it ended up not being too difficult to code one up, with the specific behavior we need.
// Basically, we start a bunch (constant number) of std::thread threads, and store them in m_threads.
// We also store a deque (double-ended queue) of work to do, where a unit of work is some arbitrary std::function<void()>.  Work is performed in roughly FIFO order.
// Each thread runs one work item at a time.  When it finished it tries to get another unit of work, or goes to sleep.
// Work is added to the queue via the add_task method.
// Note that there is only one queue of work, and all operations to it (pushing new work and popping work off to perform it) require mutex operations.  
// This may be a significant amount of overhead if the size of the work is very small and the number of work items is very large.  So for example, if the work is to copy
// a million very small files, it will be very bad to push the copy of each file to the threadpool as a separate item.  Much better to chunk it externally.
class thread_pool {
public:

    // Loop that each thread runs, to pop work off the queue and execute it.
    void run_thread(int thread_id)
    {
        // m_finished is set during threadpool destruction, allowing the threads to all exit gracefully.
        while (!m_finished)
        {
            std::function<void()> work;
            {
                // Lock the mutex for the duration of the queue operations, but not performing the actual work.
                std::unique_lock<std::mutex> lk(m_mutex);
                m_wait_count++;
                // Release the mutex and perhaps sleep until woken up and there is work to be done (or tearing down the threadpool.)
                m_cv.wait(lk, [this] {return (!m_task_list.empty()) || m_finished;});
                m_wait_count--;
                if (!m_finished)
                {
                    // Pop a unit of work from the queue; we still hold the mutex at this point.
                    work = m_task_list.front();
                    m_task_list.pop_front();
                }
            }

            // Actually do the work.
            if (!m_finished)
            {
                work();
            }
        }
    }

    // Spins up the desired number of threads, each of which will immediately wait on the condition variable.
    thread_pool(int size)
     : m_threads(), m_task_list(), m_mutex(), m_cv(), m_wait_count(0), m_finished(0)
    {
        for (int i = 0; i < size; i++)
        {
            m_threads.push_back(std::thread(&thread_pool::run_thread, this, i));
        }
    }

    // Locks the work queue, adds a task to it, and then wakes up a worker thread to do the work.
    // If no worker threads are waiting, notify_one is a no-op.
    void add_task(std::function<void()> task)
    {
        std::lock_guard<std::mutex> lk(m_mutex);
        {
            m_task_list.push_back(task);
        }
        m_cv.notify_one();
    }

    // Blocks until all work in the queue has been completed.
    // This condition is detected 
    void drain()
    {
        while (true)
        {
            {
                std::lock_guard<std::mutex> lk(m_mutex);
                if ((m_wait_count == m_threads.size()) && (m_task_list.empty()))
                {
                    return;
                }
            }
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
    }

    // Drains all the work from the pool, instructs all worker threads to wake up and exit gracefully.
    ~thread_pool()
    {
        drain();
        m_finished = true;
        m_cv.notify_all();
        for (auto it = m_threads.begin(); it != m_threads.end(); it++)
        {
            it->join();
        }
    }

    std::vector<std::thread> m_threads;
    std::deque<std::function<void()>> m_task_list;
    std::mutex m_mutex;
    std::condition_variable m_cv;
    int m_wait_count;

    bool m_finished;

};

// Callback function for destroy_path (below).
int rm_helper(const char *fpath, const struct stat * /*sb*/, int tflag, struct FTW * /*ftwbuf*/)
{
    if (tflag == FTW_DP)
    {
        errno = 0;
        int ret = rmdir(fpath);
        return ret;
    }
    else
    {
        errno = 0;
        int ret = unlink(fpath);
        return ret;
    }
}

// Delete the entire contents of a path.  FTW is a simple library for walking a directory structure.  It's fairly limited in what it can do, but it works for deletion.
void destroy_path(std::string path_to_destroy)
{
    errno = 0;
    // FTW_DEPTH instructs FTW to do a post-order traversal (children of a directory before the actual directory.)
    nftw(path_to_destroy.c_str(), rm_helper, 20, FTW_DEPTH); 
}

// Class used for running a single perf test.
// At the moment, a single perf test consists of copying the entire (recursive) contents of a directory to the service, and then copying it back.
// There are other interesting stress & performance analyses we could run as well (many writers / readers to 
// one file, for example), but we don't have infra for that yet.
// 
// Parameters for a single test are defined by the "populate" function, input in the constructor.  This 
// function is expected to create some directory structure, to use as a source directory.
// The populate method is called in the constructor.  This is because it's not really part of the actual test, although we could move it into run().
// 
// run() runs the actual test.  That consists of the following steps:
//    - Run a recursive copy from the source directory to the cloud directory, and time this operation.
//    - Run a recusrive copy from the cloud directory to the local destination directory, and time this operation.
//    - Print the relevant performance metrics
//    - Validate that the contents of both the cloud and the local destination directory match the source.

// Note: There are some important blobfuse-related parameters that are not captured here, but can have a large impact on performace:
//    - blobfuse cache timeout - if the files are still cached during download, download will be much faster.
//    - location of the blobfuse tmp directory (local SSD?  Azure Disk?  etc)
//    - Size of the VM on which blobfuse is mounted and this test runs - CPUs, Memory, etc
// For any perf tests where we save the results for later comparison, we should document as many of these as possible.
// We could also test multiple versions of blobfuse at the same time, if there are concerns that it might be hard to repro an environment.
class perf_test
    {
public:

    perf_test(int parallel_count, std::function<std::pair<size_t, size_t>(std::string, thread_pool&)> populate, std::string source_dir, std::string dest_dir_1, std::string dest_dir_2)
    : m_thread_pool(parallel_count), m_source_dir(source_dir), m_dest_dir_1(dest_dir_1), m_dest_dir_2(dest_dir_2)
    {
        std::pair<size_t, size_t> totals = populate(source_dir, m_thread_pool);
        m_total_size = totals.first;
        m_total_files = totals.second;
    }

    // We can remove the cleanup temporarily to help debugging, if necessary.
    ~perf_test()
    {
        std::cout << "Deleting test files." << std::endl;
        destroy_path(m_source_dir);
        destroy_path(m_dest_dir_1);
        destroy_path(m_dest_dir_2);
    }

thread_pool m_thread_pool;
std::string m_source_dir;
std::string m_dest_dir_1;
std::string m_dest_dir_2;
size_t m_total_size;
size_t m_total_files;

    // Print currnt time to the command line.
    // Helpful for keeping track of perf tests - if you expect a test to take an hour, come back in a while and don't remember when you started it, for example.
    void print_now()
    {
        std::time_t start = std::chrono::high_resolution_clock::to_time_t(std::chrono::high_resolution_clock::now());
        std::cout << "Now = " << std::ctime(&start) << std::endl;
    }

    // Run the test, end-to-end.
    void run()
    {
        std::cout << "About to call copy_recursive to upload." << std::endl;
        std::chrono::time_point<std::chrono::high_resolution_clock> start_upload = std::chrono::high_resolution_clock::now();
        copy_recursive(m_source_dir, m_dest_dir_1);
        m_thread_pool.drain();  // Note that we have to wait for the pool to drain before stopping the clock.
        std::chrono::time_point<std::chrono::high_resolution_clock> end_upload = std::chrono::high_resolution_clock::now();
        double upload_time = std::chrono::duration_cast<std::chrono::microseconds>(end_upload - start_upload).count() / (1000000.0);

        std::cout << "Upload finished." << std::endl;

        print_now();

        std::cout << "About to call copy_recursive to download." << std::endl;
        std::chrono::time_point<std::chrono::high_resolution_clock> start_download = std::chrono::high_resolution_clock::now();
        copy_recursive(m_dest_dir_1, m_dest_dir_2);
        m_thread_pool.drain();
        std::chrono::time_point<std::chrono::high_resolution_clock> end_download = std::chrono::high_resolution_clock::now();
        double download_time = std::chrono::duration_cast<std::chrono::microseconds>(end_download - start_download).count() / (1000000.0);
        std::cout << "Download finished." << std::endl;

        double mbps_up = (m_total_size * 8) / (upload_time * 1024 * 1024);  // Note we calculate Mb, not MB.
        double mbps_down = (m_total_size * 8) / (download_time * 1024 * 1024);

        std::cout << "Upload took " << upload_time << " seconds, averaging " << mbps_up << "Mb per second." << std::endl;
        std::cout << "Download took " << download_time << " seconds, averaging " << mbps_down << "Mb per second." << std::endl;

        print_now();

        std::cout << "Now validating." << std::endl;
        validate_directory(m_source_dir, m_dest_dir_1);
        m_thread_pool.drain();
        validate_directory(m_source_dir, m_dest_dir_2);
        m_thread_pool.drain();

        std::cout << "Contents validated." << std::endl;
    }

    // Helper function to copy a file.
    // sendfile() is used to avoid having to copy data into userspace (other than in blobfuse, of course) - read() and write() would have additional user-space copies.
    void copy_file(std::string input, std::string output)
    {
    //    std::cout << "Operate file called with " << input << " and " << output << std::endl;
        int input_fd = open(input.c_str(), O_RDONLY);
        if (input_fd < 0)
        {
            std::stringstream error;
            error << "Failed to open input file.  errno = " << errno << ", input = " << input;
            throw std::runtime_error(error.str());
        }
        int output_fd = open(output.c_str(), O_WRONLY | O_CREAT | O_EXCL, 0777);
        if (input_fd < 0)
        {
            std::stringstream error;
            error << "Failed to open output file.  errno = " << errno << ", output = " << output;
            throw std::runtime_error(error.str());
        }

        struct stat st;
        stat(input.c_str(), &st);
        size_t count = st.st_size;
        size_t initial_size = count;

        while (count > 0)
        {
            ssize_t copied = sendfile(output_fd, input_fd, NULL, count);
            if (copied < 0)
            {
                std::stringstream error;
                error << "Failed to copy file.  errno = " << errno << ", input = " << input << ", initial size = " << initial_size << ", bytes remaining = " << count;
                throw std::runtime_error(error.str());
            }
            else
            {
                count -= copied;
            }
        }

        close(input_fd);
        close(output_fd);
    }

    // Helper function to list all files in a directory, and call some other function for each file.
    // Used during directory validation.
    void list_in_directory(std::string dir_to_list, std::function<void (struct dirent*)> dir_ent_op)
    {
        DIR *dir_stream = opendir(dir_to_list.c_str());
        if (dir_stream != NULL)
        {
            struct dirent* dir_ent = readdir(dir_stream);
            while (dir_ent != NULL)
            {
                if (dir_ent->d_name[0] != '.')
                {
                    dir_ent_op(dir_ent);
                }
                dir_ent = readdir(dir_stream);
            }
            closedir(dir_stream);
        }
        else
        {
            std::stringstream error;
            error << "Failed to open directory.  errno = " << errno << ", directory = " << dir_to_list;
            throw std::runtime_error(error.str());
        }
    }

    // Validate that the contents of two files match.
    void validate_file(std::string input, std::string output)
    {
        std::ifstream input_file(input, std::ifstream::ate | std::ifstream::binary);
        std::ifstream output_file(output, std::ifstream::ate | std::ifstream::binary);

        if (input_file.tellg() != output_file.tellg())
        {
            std::stringstream error;
            error << "Files are not the same size.  File " << input << " has size " << input_file.tellg() << ", file " << output << " has size " << output_file.tellg();
            throw std::runtime_error(error.str());
        }
        size_t count = input_file.tellg();

        input_file.seekg(0, std::ifstream::beg);
        output_file.seekg(0, std::ifstream::beg);
        uint read_buf_size = 1*1024*1024;
        char inputbuf[read_buf_size];
        char outputbuf[read_buf_size];

        while (count > 0)
        {
            uint size_to_read = read_buf_size < count ? read_buf_size : count;
            input_file.read(inputbuf, size_to_read);
            output_file.read(outputbuf, size_to_read);

            if (0 != memcmp(inputbuf, outputbuf, size_to_read))
            {
                std::stringstream error;
                error << "File contents do not match.  Files are " << input << " and " << output;
                throw std::runtime_error(error.str());
            }

            count -= size_to_read;
        }
    }

    // Recursively copy one directory to another directory.
    // Note that file copies are done serially, as they come up, while directory copies are added to the threadpool.
    // This must be kept in mind when designing file structures to copy.
    void copy_recursive(std::string input_dir, std::string output_dir)
    {
        struct stat st;
        if (stat(output_dir.c_str(), &st) != 0) {
            int mkdirret = mkdir(output_dir.c_str(), 0777);
            if (mkdirret < 0)
            {
                //std::stringstream error;
                //error << "Failed to make directory.  errno = " << errno << ", directory = " << output_dir;
                //throw std::runtime_error(error.str());
            }
        }

        DIR *dir_stream = opendir(input_dir.c_str());
        if (dir_stream != NULL)
        {
            struct dirent* dir_ent = readdir(dir_stream);
            while (dir_ent != NULL)
            {
                if (dir_ent->d_name[0] != '.')
                {
                    std::string input(input_dir + "/" + dir_ent->d_name);
                    std::string output(output_dir + "/" + dir_ent->d_name);
                    if (dir_ent->d_type == DT_DIR)
                    {
                        m_thread_pool.add_task([this, input, output] () {
                            copy_recursive(input, output);
                        });
                    }
                    else
                    {
                        copy_file(input, output);
                    }
                }
                dir_ent = readdir(dir_stream);
            }
            closedir(dir_stream);
        }
        else
        {
            std::stringstream error;
            error << "Failed to open directory.  errno = " << errno << ", directory = " << input_dir;
            throw std::runtime_error(error.str());
        }
    }

    // Validate that the contents of two directories are equal.
    // Does file comparisons serially; adds directory comparisons to the threadpool.
    void validate_directory(std::string input_dir, std::string output_dir)
    {
        std::vector<std::string> input_file_list;
        std::vector<std::string> input_directory_list;
        std::vector<std::string> output_file_list;
        std::vector<std::string> output_directory_list;

        list_in_directory(input_dir, [&input_file_list, &input_directory_list] (struct dirent* dir_ent) {
            std::string name(dir_ent->d_name);
            if (dir_ent->d_type == DT_DIR)
            {
                input_directory_list.push_back(name);
            }
            else
            {
                input_file_list.push_back(name);
            }
        });

        list_in_directory(output_dir, [&output_file_list, &output_directory_list] (struct dirent* dir_ent) {
            if (dir_ent->d_type == DT_DIR)
            {
                output_directory_list.push_back(dir_ent->d_name);
            }
            else
            {
                output_file_list.push_back(dir_ent->d_name);
            }
        });

        std::sort(input_file_list.begin(), input_file_list.end());
        std::sort(input_directory_list.begin(), input_directory_list.end());
        std::sort(output_file_list.begin(), output_file_list.end());
        std::sort(output_directory_list.begin(), output_directory_list.end());

        if (input_file_list != output_file_list)
        {
            std::stringstream error;
            error << "List of files in directories do not match.  Left dir = " << input_dir << ", right dir = " << output_dir;
            throw std::runtime_error(error.str());
        }

        if (input_directory_list != output_directory_list)
        {
            std::stringstream error;
            error << "List of subdirectories in directories do not match.  Left parent dir = " << input_dir << ", right parent dir = " << output_dir;
            throw std::runtime_error(error.str());
        }

        for (int i = 0; i < input_directory_list.size(); i++)
        {
            std::string input_subdir(input_dir + "/" + input_directory_list[i]);
            std::string output_subdir(output_dir + "/" + output_directory_list[i]);
            m_thread_pool.add_task([this, input_subdir, output_subdir] () {
                validate_directory(input_subdir, output_subdir);
            });
        }

        for (int i = 0; i < input_file_list.size(); i++)
        {
            std::string input_file(input_dir + "/" + input_file_list[i]);
            std::string output_file(output_dir + "/" + output_file_list[i]);
            validate_file(input_file, output_file);
        }
    }

};

// Helper method to print out relevant information for each test run.
// May need to change as additional scenarios are added.
void print_test_initial_stats(int total_dir_count, int file_per_dir_count, size_t file_size_base, long unsigned int additional_size_jitter)
{
    std::cout << "Total directory count = " << total_dir_count << ", files per directory = " << file_per_dir_count << "." << std::endl;
    std::cout << "File sizes chosen from roughly random uniform distribution between " << file_size_base << " and " << file_size_base + additional_size_jitter << " bytes." << std::endl;
    std::cout << "This adds up to around " << total_dir_count * file_per_dir_count * (file_size_base + (additional_size_jitter/2)) << " bytes total, across " << total_dir_count * file_per_dir_count << " files." << std::endl;
}

// Creates a directory structure for testing copies of large files.
// Creates one file per directory, so that each file is copied in parallel.
// TODO: remove duplicated logic between populate_* methods.
std::pair<size_t, size_t> populate_large(std::string source_dir, thread_pool& pool)
{
    #if 0
    int total_dir_count = 30; 
    size_t file_size_base = 50*1024*1024;  // Each file will be roughly 50 MB in size (increase this for actual perf testing)
    long unsigned int additional_size_jitter = 1024 * 1024;  // Each file will have between 0-1MB added to it (on top of the 500 MB))
    #else
    int total_dir_count = 10; 
    size_t file_size_base = 10*1024*1024; 
    long unsigned int additional_size_jitter = 100;  // Each file will have between 0-1MB added to it (on top of the 500 MB))
    #endif

    int seed = 4;  // We use a constant seed here to make each run identical; this probably doesn't matter a ton.
    std::minstd_rand r(seed);  // minstd_rand has terrible randomness properties, but it's more than good enough for our purposes here, and is far faster than better options.
    size_t total_size = 0;
     
    std::cout << "Running large file stress test." << std::endl;
    print_test_initial_stats(total_dir_count, 1, file_size_base, additional_size_jitter);

    for (int i = 0; i < total_dir_count; i++)
    {
        std::string dir = source_dir + "/" + std::to_string(i);
        int mkdirret = mkdir(dir.c_str(), 0777);
        if (mkdirret < 0)
        {
            //std::stringstream error;
            //error << "Failed to make directory.  errno = " << errno << ", directory = " << dir;
            //throw std::runtime_error(error.str());
        }

        std::string file = dir + "/file";
        uint_fast32_t start = r();
        size_t file_size = file_size_base + (r() % additional_size_jitter);
        total_size += file_size;
        pool.add_task([=] () {
            uint_fast32_t current = start;
            std::ofstream file_stream(file, std::ios::binary);
            for (size_t i = 0; i < file_size; i += 4 /* sizeof uint_fast32_t */)
            {
                file_stream.write(reinterpret_cast<char*>(&current), 4);
                current++;
            }
        });
    }

    pool.drain();
    return std::make_pair(total_size, total_dir_count);
}

// Creates a directory structure for testing copying large numbers of small files.
// Directories are copied in parallel to each other, while files in a directory are copied serially.  This must be considered when choosing parameters.
std::pair<size_t, size_t> populate_small(std::string source_dir, thread_pool& pool)
{
    int seed = 4;
    std::minstd_rand r(seed);
    #if 0
    size_t file_size_base = 1024;  // Each file has a base size of 1 KB.
    long unsigned int additional_size_jitter = 1024;  // Each file will have between 0-1KB added to it, randomly.
    int total_dir_count = 60;  
    int file_per_dir_count = 10000; 
    #else
    size_t file_size_base = 1024;  // Each file has a base size of 1 KB.
    long unsigned int additional_size_jitter = 10;  // Each file will have between 0-1KB added to it, randomly.
    int total_dir_count = 10; 
    int file_per_dir_count = 100;  
    #endif
    size_t total_size = total_dir_count * file_per_dir_count * (file_size_base + (additional_size_jitter/2));  // Here we just estimate the total size, more than close enough.
    std::cout << "Running small file stress test." << std::endl;
    print_test_initial_stats(total_dir_count, file_per_dir_count, file_size_base, additional_size_jitter);

    for (int i = 0; i < total_dir_count; i++)
    {
        std::string dir = source_dir + "/" + std::to_string(i);
        int mkdirret = mkdir(dir.c_str(), 0777);
        if (mkdirret < 0)
        {
            //std::stringstream error;
            //error << "Failed to make directory.  errno = " << errno << ", directory = " << dir;
            //throw std::runtime_error(error.str());
        }

        std::string file = dir + "/file";

        uint_fast32_t r_local_seed = r();
        pool.add_task([=] () {
            std::minstd_rand r_local(r_local_seed);
            for (int j = 0; j < file_per_dir_count; j++)
            {
                std::stringstream file_name_stream;
                file_name_stream << file << std::setfill('0') << std::setw(8) << j;
                size_t file_size = file_size_base + (r_local() % additional_size_jitter);
                uint_fast32_t current = r_local();
                std::ofstream file_stream(file_name_stream.str(), std::ios::binary);
                for (size_t i = 0; i < file_size; i += 4 /* sizeof uint_fast32_t */)
                {
                    file_stream.write(reinterpret_cast<char*>(&current), 4);
                    current++;
                }
            }
        });
    }

    pool.drain();
    return std::make_pair(total_size, total_dir_count * file_per_dir_count);
}

int main(int argc, char *argv[])
{
	signal(SIGINT, signalHandler);
  
    if (argc >= 3) {
        uuid_t dir_uuid;
        uuid_generate( (unsigned char *)&dir_uuid );

        char dir_name_uuid[37];
        uuid_unparse_lower(dir_uuid, dir_name_uuid);
        
        std::string dir_name_prefix = "stresstest";
        std::string dir_name = dir_name_prefix + dir_name_uuid;

        perf_source_dir = std::string(argv[2]) + "/src";  
        perf_dest_dir_1 = std::string(argv[1]) + "/" + dir_name;
        perf_dest_dir_2 = std::string(argv[2]) + "/dst"; 
        printf("Running with : MNT : %s, SRC : %s, DST : %s\n", \
                perf_dest_dir_1.c_str(), perf_source_dir.c_str(), perf_dest_dir_2.c_str());
        //return 0;
    } else {
        printf("\nUsage : blobfusestress <mounted-dir> <tmp-download-dir>\n\n");
        return 0;
    }

    try
    {
        std::vector<std::function<std::pair<size_t, size_t>(std::string, thread_pool&)>> populate_fns
        {
            populate_small,
            populate_large,
        };

        std::cout << populate_fns.size() << " tests to run in total." << std::endl << std::endl;
        for (int i = 0; i < populate_fns.size(); i++)
        {
            std::cout << std::endl << "Starting test " << i << "." << std::endl;
            std::function<std::pair<size_t, size_t>(std::string, thread_pool&)> populate_func = populate_fns[i];

            std::time_t start = std::chrono::high_resolution_clock::to_time_t(std::chrono::high_resolution_clock::now());
            std::cout << "Start time = " << std::ctime(&start) << std::endl;

            struct stat st;
            if (stat(perf_source_dir.c_str(), &st) != 0) {
                int mkdirret = mkdir(perf_source_dir.c_str(), 0777);
                /*if (mkdirret < 0)
                {
                    std::stringstream error;
                    error << "Failed to make directory.  errno = " << errno << ", directory = " << perf_source_dir;
                    throw std::runtime_error(error.str());
                }*/
            }

            int parallel = 8; // Run 8 threads in parallel.
            std::cout << "Parallel count = " << parallel << std::endl;
            std::cout << "Starting generating test files." << std::endl;
            perf_test test(parallel, populate_func, perf_source_dir, perf_dest_dir_1, perf_dest_dir_2);

            std::cout << "Now running test." << std::endl;
            test.run();

            std::time_t end = std::chrono::high_resolution_clock::to_time_t(std::chrono::high_resolution_clock::now());
            std::cout << "End time = " << std::ctime(&end) << std::endl;
            rmdir(perf_source_dir.c_str());
        }
    }
    catch (const std::exception& e)
    {
        std::cout << "Critical error encountered.  e.what() = " << e.what() << std::endl;
    }
    return 0;
}
