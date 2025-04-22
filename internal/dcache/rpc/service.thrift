namespace go dcache.service

include "models.thrift"

// Define the service with RPC methods
service ChunkService {
    // check if the node is reachable
    models.HelloResponse Hello(1: models.HelloRequest request)

    // fetch the chunk from the node from the given fsid
    models.GetChunkResponse GetChunk(1: models.GetChunkRequest request)

    // store the chunk on the node on the given fsid
    models.PutChunkResponse PutChunk(1: models.PutChunkRequest request)

    // delete the chunk from the node from the given fsid
    models.RemoveChunkResponse RemoveChunk(1: models.RemoveChunkRequest request)

    // add RV to the given MV
    models.JoinMVResponse JoinMV(1: models.JoinMVRequest request)

    // remove RV from the given MV
    models.LeaveMVResponse LeaveMV(1: models.LeaveMVRequest request)

    models.StartSyncResponse StartSync(1: models.StartSyncRequest request)

    models.EndSyncResponse EndSync(1: models.EndSyncRequest request)
}
