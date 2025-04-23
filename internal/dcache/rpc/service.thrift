namespace go dcache.service

include "models.thrift"

// Define the service with RPC methods
service ChunkService {
    // check if the node is reachable
    models.HelloResponse Hello(1: models.HelloRequest request)

    // fetch the chunk from the node from the given rvID
    models.GetChunkResponse GetChunk(1: models.GetChunkRequest request)

    // store the chunk on the node on the given rvID
    models.PutChunkResponse PutChunk(1: models.PutChunkRequest request)

    // delete the chunk from the node from the given rvID
    models.RemoveChunkResponse RemoveChunk(1: models.RemoveChunkRequest request)

    // add RV to the given MV
    models.JoinMVResponse JoinMV(1: models.JoinMVRequest request)

    // update the component RVs for the given MV
    // this call is sent after the JoinMV call to the online RVs to update their component RVs list
    models.UpdateMVResponse UpdateMV(1: models.UpdateMVRequest request)

    // remove RV from the given MV
    models.LeaveMVResponse LeaveMV(1: models.LeaveMVRequest request)

    models.StartSyncResponse StartSync(1: models.StartSyncRequest request)

    models.EndSyncResponse EndSync(1: models.EndSyncRequest request)
}
