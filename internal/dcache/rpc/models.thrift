namespace go dcache.models

struct HelloRequest {
    1: string senderNodeID,
    2: string receiverNodeID,
    3: i64 time, // current time in usec in sender
    4: list<string> RV,
    5: list<string> MV
}

struct HelloResponse {
    1: string receiverNodeID,
    2: i64 time, // current time in usec in receiver
    3: list<string> RV,
    4: list<string> MV
}

struct Address {
    1: string fileID,
    2: string fsID,
    3: string mvID,
    4: i64 offsetInMB
}

struct Chunk {
    1: Address address,
    2: binary data,
    3: string hash,
}

struct GetChunkRequest {
    1: Address address,
    2: i64 offset, // relative offset inside chunk
    3: i64 length
}

struct GetChunkResponse {
    1: Chunk chunk,
    2: string chunkWriteTime,
    3: i64 timeTaken,
    4: list<string> peerRV
}

struct PutChunkRequest {
    1: Chunk chunk
    2: i64 length,
    3: bool isSync
}

struct PutChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<string> peerRV
}

struct RemoveChunkRequest {
    1: Address address
}

struct RemoveChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<string> peerRV
}

struct JoinMVRequest {
    1: string MV,
    2: string RV,
    3: i64 reserveSpace,
    4: list<string> peerRV
}

struct JoinMVResponse {
    // status will be returned in the error
}

struct LeaveMVRequest {
    1: string MV,
    2: string RV,
    3: list<string> peerRV
}

struct LeaveMVResponse {
    // status will be returned in the error
}

struct StartSyncRequest {
    1: string MV,
    2: list<string> peerRV,
    3: string sourceRV,
    4: string targetRV,
    5: i64 dataLength
}

struct StartSyncResponse {
    // status will be returned in the error
    1: string syncID
}

struct EndSyncRequest {
    1: string syncID,
    2: string MV,
    3: list<string> peerRV,
    4: string sourceRV,
    5: string targetRV,
    6: i64 dataLength
}

struct EndSyncResponse {
    // status will be returned in the error
}