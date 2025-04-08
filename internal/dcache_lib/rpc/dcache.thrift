namespace go dcache

// chunk object
struct Chunk {
    1: string fileID,
    2: string fsID,
    3: i64 mirrorVolume,
    4: i64 offset,
    5: i64 length,
    6: string hash,
    7: binary data
}

// Define the service with RPC methods
service ChunkService {
    // check if the node is reachable
    void Ping()

    // fetch the chunk from the node
    Chunk GetChunk(1: string fileID, 2: string fsID, 3: i64 mirrorVolume, 4: i64 offset)

    // store the chunk on the node
    void PutChunk(1: Chunk chunk)

    // delete the chunk from the node
    void RemoveChunk(1: string fileID, 2: string fsID, 3: i64 mirrorVolume, 4: i64 offset)
}
