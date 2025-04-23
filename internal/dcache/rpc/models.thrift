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
    4: list<string> componentRV
    // TODO:: discuss: should we validate the component RV in GetChunk call
}

struct PutChunkRequest {
    1: Chunk chunk
    2: i64 length,
    3: bool isSync
    4: list<string> componentRV // used to validate the component RV for the MV
}

struct PutChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<string> componentRV
}

struct RemoveChunkRequest {
    1: Address address
    // TODO:: discuss: should we validate the component RV in RemoveChunk call
}

struct RemoveChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<string> componentRV
}

struct JoinMVRequest {
    1: string MV,
    2: string RV,
    3: i64 reserveSpace,
    4: list<string> componentRV
}

struct JoinMVResponse {
    // status will be returned in the error
}

struct LeaveMVRequest {
    1: string MV,
    2: string RV,
    3: list<string> componentRV
}

struct LeaveMVResponse {
    // status will be returned in the error
}

struct StartSyncRequest {
    1: string MV,
    2: string sourceRV, // source RV is the lowest index online RV. The node hosting this RV will send the start sync call to the component RVs
    3: string targetRV, // target RV is the target of the start sync request
    4: list<string> componentRV,
    5: i64 dataLength
}

struct StartSyncResponse {
    // status will be returned in the error
    1: string syncID
}

struct EndSyncRequest {
    1: string syncID,
    2: string MV,
    3: string sourceRV, // source RV is the lowest index online RV. The node hosting this RV will send the end sync call to the component RVs
    4: string targetRV, // target RV is the RV which has to stop the sync marking it as completed
    5: list<string> componentRV,
    6: i64 dataLength
}

struct EndSyncResponse {
    // status will be returned in the error
}