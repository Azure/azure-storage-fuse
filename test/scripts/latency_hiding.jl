using TickTock
using .Libc

sleep_time = parse(Float64, ARGS[1])

block_size = 64 * 1024 * 1024 #bytes
file_path = "/tmp/mntpoint1/sample31"

block_count = Int(floor(filesize(file_path) / block_size))
in_data = open(file_path, "r")
buffer = Array{UInt8, 1}(undef, block_size)
computation_time = 0.0
runtime = 0.0
readtime = 0.0
dummy = 0
println("SleepTime: ", sleep_time)
println("Bytewise XOR of ", Int(block_count), " blocks of size ", Int(block_size), "B")

tick() #start timer
for i = 1:block_count
    global dummy, computation_time, runtime, readtime, sleep_time
    t0 = time_ns()
    read!(in_data, buffer)
    t1 = time_ns()
    dummy = xor(dummy, reduce(xor, buffer))
    Libc.systemsleep(sleep_time) #additional calculation time in seconds
    t2 = time_ns()
    readtime += t1-t0
    computation_time += t2-t1
    println("Instantaneous readTime: ", t1-t0, "ns")
    runtime += t2-t0
end
tock() #stop timer and output time difference

close(in_data)
println("cummalative_read_time=", readtime/1.0e9, "s")
println("cummalative_computation_time=", computation_time/1.0e9, "s")
println("cummalative_runtime=", runtime/1.0e9, "s")
println("xor=", dummy) #output dummy to keep JIT compiler from removing xor and read

timing_output = open("latency_hiding.dat", "a")
write(timing_output, string(runtime/1.0e9) * " " * string(computation_time/1.0e9) * " " * string(readtime/1.0e9) * "\n")
close(timing_output)