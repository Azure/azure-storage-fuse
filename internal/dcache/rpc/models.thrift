namespace go dcache.models

struct HelloRequest {
    1: string senderNodeID,
    2: string receiverNodeID,
    3: i64 time, // current time in usec in sender
    4: list<string> RVName,
    5: list<string> MV
    6: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct HelloResponse {
    1: string receiverNodeID,
    2: i64 time, // current time in usec in receiver
    3: list<string> RVName,
    4: list<string> MV
    5: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
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
    5: bool isLocalRV, // true, if both server and client are on the same node
    6: list<RVNameAndState> componentRV // used to validate the component RV for the MV
    7: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct GetChunkResponse {
    1: Chunk chunk,
    2: string chunkWriteTime,
    3: i64 timeTaken,
    4: list<RVNameAndState> componentRV
    5: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct PutChunkRequest {
    1: string senderNodeID,
    2: Chunk chunk,
    3: i64 length,
    4: string syncID, // only valid for PutChunk(sync) calls, syncID of the ongoing sync operation
    5: string sourceRVName, // only valid for PutChunk(sync) calls, source RV from which data is being synced
    6: list<RVNameAndState> componentRV, // used to validate the component RV for the MV
    7: bool maybeOverwrite
    8: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct PutChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 qsize, // request queue size at the RV server when this response is sent
    3: i64 availableSpace,
    4: list<RVNameAndState> componentRV
    5: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct PutChunkDCRequest {
    1: PutChunkRequest request,
    2: list<string> nextRVs
}

// Type for the individual PutChunkResponse or error.
struct PutChunkResponseOrError {
    1: optional PutChunkResponse response,
    2: optional ResponseError error
}

struct PutChunkDCResponse {
    1: map<string, PutChunkResponseOrError> responses // map of RV name to the PutChunk response or error to that RV
    2: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

// Remove chunks belonging to a file.
struct RemoveChunkRequest {
    1: string senderNodeID,
    2: Address address,
    3: list<RVNameAndState> componentRV // used to validate the component RV for the MV
    4: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct RemoveChunkResponse {
    // status will be returned in the error
    1: i64 timeTaken,
    2: i64 availableSpace,
    3: list<RVNameAndState> componentRV,
    //
    // Total number of chunks deleted by this request.
    // When a RemoveChunkResponse carries a status of success and numChunksDeleted==0, it would indicate
    // to the caller that all chunks of the file are deleted from the specified rv/mv directory.
    //
    4: i64 numChunksDeleted,
    5: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct JoinMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: i64 reserveSpace,
    5: list<RVNameAndState> componentRV
    6: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct JoinMVResponse {
    // status will be returned in the error
    1: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct UpdateMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: list<RVNameAndState> componentRV
    5: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct UpdateMVResponse {
    // status will be returned in the error
    1: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct LeaveMVRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName,
    4: list<RVNameAndState> componentRV
    5: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct LeaveMVResponse {
    // status will be returned in the error
    1: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

struct GetMVSizeRequest {
    1: string senderNodeID,
    2: string MV,
    3: string RVName
    4: i64 clustermapEpoch, // Sender's clustermap epoch when the request is sent
}

struct GetMVSizeResponse {
    1: i64 mvSize
    2: i64 clustermapEpoch, // Receiver's clustermap epoch when the response is sent
}

//
// Request to initiate or continue log collection transfer.
// The client will call GetLogs RPC repeatedly with increasing chunkIndex starting at 0.
// Server will create (on first chunkIndex==0) a tar.gz containing all blobfuse2.log* files
// from its log directory and stream it back in 16MB chunks until isLast=true in response.
//
struct GetLogsRequest {
    1: string senderNodeID,
    2: i64 chunkIndex, // zero-based chunk index requested
    3: i64 numLogs, // collect atmost this number of most recent logs from each node
    4: i64 chunkSize, // desired chunk size in bytes
}

struct GetLogsResponse {
    1: binary data, // log tarball chunk bytes
    2: i64 chunkIndex,
    3: bool isLast, // true if this is the final chunk
    4: i64 totalSize, // total size of tarball in bytes
    5: string tarName // name of tarball file on server (e.g., <nodeID>-blobfuse2-logs-<time_RFC3339>.tar.gz)
}

struct GetNodeStatsRequest {
    1: string senderNodeID,
}

struct GetNodeStatsResponse {
    1: string timestamp,
    2: string nodeID,
    3: string hostName,
    4: i64 memUsedBytes,
    5: i64 memTotalBytes,
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
    NeedToRefreshClusterMap = 8,
    ThriftError = 9,
    BrokenChain = 10
}

// Custom error returned by the RPC APIs
exception ResponseError {
    1: ErrorCode code,
    2: string message
}
