namespace go dcache.models

struct HelloRequest {
    1: string senderNodeID,
    2: string receiverNodeID,
    3: i64 time, // current time in usec in sender
    4: list<string> RVName,
    5: list<string> MV
}

struct HelloResponse {
    1: string receiverNodeID,
    2: i64 time, // current time in usec in receiver
    3: list<string> RVName,
    4: list<string> MV
}

struct Address {
    1: string fileID,
    2: string rvID,
    3: string mvName,
    4: i64 offsetInMiB
}

struct Chunk {
    1: Address address,
    2: binary data,
    3: string hash
}

struct RVNameAndState {
    1: string name,
    2: string state
}

struct GetChunkRequest {
    1: string senderNodeID,
    2: Address address,
    3: i64 offsetInChunk,
    4: i64 length,
    5: list<RVNameAndState> componentRV // used to validate the component RV for the MV
}

struct GetChunkResponse {
    1: Chunk chunk,
    2: string chunkWriteTime,
    3: i64 timeTaken,
    4: list<RVNameAndState> componentRV
}

struct PutChunkRequest {
    1: string senderNodeID,
    2: Chunk chunk,
    3: i64 length,
    4: string syncID,
    5: list<RVNameAndState> componentRV, // used to validate the component RV for the MV
    6: bool maybeOverwrite
}

struct PutChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<RVNameAndState> componentRV
}

// Remove chunks belonging to a file.
struct RemoveChunkRequest {
    1: string senderNodeID,
    2: Address address,
    3: list<RVNameAndState> componentRV // used to validate the component RV for the MV
}

struct RemoveChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<RVNameAndState> componentRV,
    //
    // Total number of chunks deleted by this request.
    //
    4: i64 numChunksDeleted,
    //
    // The following flag is set when not all chunks to the corresponding fileID are not deleted from the RV. The
    // caller needs to retry the request in this case.
    //
    5: bool needRetry
}

struct JoinMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: i64 reserveSpace,
    5: list<RVNameAndState> componentRV
}

struct JoinMVResponse {
    // status will be returned in the error
}

struct UpdateMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: list<RVNameAndState> componentRV
}

struct UpdateMVResponse {
    // status will be returned in the error
}

struct LeaveMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: list<RVNameAndState> componentRV
}

struct LeaveMVResponse {
    // status will be returned in the error
}

struct StartSyncRequest {
    1: string senderNodeID,
    2: string MV,
    3: string sourceRVName, // source RV is the lowest index online RV. The node hosting this RV will send the start sync call to the component RVs
    4: string targetRVName, // target RV is the target of the start sync request
    5: list<RVNameAndState> componentRV,
    6: i64 syncSize
}

struct StartSyncResponse {
    // status will be returned in the error
    1: string syncID
}

struct EndSyncRequest {
    1: string senderNodeID,
    2: string syncID,
    3: string MV,
    4: string sourceRVName, // source RV is the lowest index online RV. The node hosting this RV will send the end sync call to the component RVs
    5: string targetRVName, // target RV is the RV which has to stop the sync marking it as completed
    6: list<RVNameAndState> componentRV,
    7: i64 syncSize
}

struct EndSyncResponse {
    // status will be returned in the error
}

struct GetMVSizeRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName
}

struct GetMVSizeResponse {
    1: i64 mvSize
}

// Custom error codes returned by the ChunkServiceHandler
enum ErrorCode {
    InvalidRequest = 1,
    InvalidRVID = 2,
    InvalidRV = 3,
    InternalServerError = 4,
    ChunkNotFound = 5,
    ChunkAlreadyExists = 6,
    MaxMVsExceeded = 7,
    NeedToRefreshClusterMap = 8
}

// Custom error returned by the RPC APIs
exception ResponseError {
    1: ErrorCode code,
    2: string message
}
