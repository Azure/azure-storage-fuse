namespace go dcache.service

include "models.thrift"

// Define the service with RPC methods
service ChunkService {
    // check if the node is reachable
    models.HelloResponse Hello(1: models.HelloRequest request) throws (1:models.ResponseError err)

    // fetch the chunk from the node from the given rvID
    models.GetChunkResponse GetChunk(1: models.GetChunkRequest request) throws (1:models.ResponseError err)

    // store the chunk on the node on the given rvID
    models.PutChunkResponse PutChunk(1: models.PutChunkRequest request) throws (1:models.ResponseError err)

    // delete the chunk from the node from the given rvID
    models.RemoveChunkResponse RemoveChunk(1: models.RemoveChunkRequest request) throws (1:models.ResponseError err)

    // add RV to the given MV
    models.JoinMVResponse JoinMV(1: models.JoinMVRequest request) throws (1:models.ResponseError err)

    // update the component RVs for the given MV
    // this call is sent after the JoinMV call to the online RVs to update their component RVs list
    models.UpdateMVResponse UpdateMV(1: models.UpdateMVRequest request) throws (1:models.ResponseError err)

    // remove RV from the given MV
    models.LeaveMVResponse LeaveMV(1: models.LeaveMVRequest request) throws (1:models.ResponseError err)

    models.StartSyncResponse StartSync(1: models.StartSyncRequest request) throws (1:models.ResponseError err)

    models.EndSyncResponse EndSync(1: models.EndSyncRequest request) throws (1:models.ResponseError err)

    // retrieve the size of the specified MV
    models.GetMVSizeResponse GetMVSize(1: models.GetMVSizeRequest request) throws (1:models.ResponseError err)
}
