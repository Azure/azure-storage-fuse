/*
    _____           _____   _____   ____          ______  _____  ------
   |     |  |      |     | |     | |     |     | |       |            |
   |     |  |      |     | |     | |     |     | |       |            |
   | --- |  |      |     | |-----| |---- |     | |-----| |-----  ------
   |     |  |      |     | |     | |     |     |       | |       |
   | ____|  |_____ | ____| | ____| |     |_____|  _____| |_____  |_____


   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.
   Author : <blobfusedev@microsoft.com>

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package rpc_server

import (
	"context"
	"errors"
	"time"

	grpcmodels "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/models"
	grpcservice "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go-grpc/service"
	thriftmodels "github.com/Azure/azure-storage-fuse/v2/internal/dcache/rpc/gen-go/dcache/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcChunkService implements the generated gRPC interface and delegates to existing handler
// TODO: Complete conversions for all RPCs.
type grpcChunkService struct {
	grpcservice.UnimplementedChunkServiceServer
}

func (g *grpcChunkService) Hello(ctx context.Context, req *grpcmodels.HelloRequest) (*grpcmodels.HelloResponse, error) {
	if handler == nil { // global from handler.go
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.HelloRequest{
		SenderNodeID:    req.SenderNodeID,
		ReceiverNodeID:  req.ReceiverNodeID,
		Time:            req.Time,
		RVName:          req.RVName,
		MV:              req.MV,
		ClustermapEpoch: req.ClustermapEpoch,
	}
	tResp, err := handler.Hello(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.HelloResponse{
		ReceiverNodeID:  tResp.ReceiverNodeID,
		Time:            tResp.Time,
		RVName:          tResp.RVName,
		MV:              tResp.MV,
		ClustermapEpoch: tResp.ClustermapEpoch,
	}, nil
}

func (g *grpcChunkService) GetChunk(ctx context.Context, req *grpcmodels.GetChunkRequest) (*grpcmodels.GetChunkResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.GetChunkRequest{
		SenderNodeID:    req.SenderNodeID,
		Address:         convertAddressPbToThrift(req.Address),
		OffsetInChunk:   req.OffsetInChunk,
		Length:          req.Length,
		IsLocalRV:       req.IsLocalRV,
		ComponentRV:     convertRVListPbToThrift(req.ComponentRV),
		ClustermapEpoch: req.ClustermapEpoch,
	}
	tResp, err := handler.GetChunk(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.GetChunkResponse{
		Chunk:           convertChunkThriftToPb(tResp.Chunk),
		ChunkWriteTime:  tResp.ChunkWriteTime,
		TimeTaken:       tResp.TimeTaken,
		ComponentRV:     convertRVListThriftToPb(tResp.ComponentRV),
		ClustermapEpoch: tResp.ClustermapEpoch,
	}, nil
}
func (g *grpcChunkService) PutChunk(ctx context.Context, req *grpcmodels.PutChunkRequest) (*grpcmodels.PutChunkResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.PutChunkRequest{
		SenderNodeID:    req.SenderNodeID,
		Chunk:           convertChunkPbToThrift(req.Chunk),
		Length:          req.Length,
		SyncID:          req.SyncID,
		SourceRVName:    req.SourceRVName,
		ComponentRV:     convertRVListPbToThrift(req.ComponentRV),
		MaybeOverwrite:  req.MaybeOverwrite,
		ClustermapEpoch: req.ClustermapEpoch,
	}
	tResp, err := handler.PutChunk(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.PutChunkResponse{
		TimeTaken:       tResp.TimeTaken,
		AvailableSpace:  tResp.AvailableSpace,
		ComponentRV:     convertRVListThriftToPb(tResp.ComponentRV),
		ClustermapEpoch: tResp.ClustermapEpoch,
	}, nil
}
func (g *grpcChunkService) PutChunkDC(ctx context.Context, req *grpcmodels.PutChunkDCRequest) (*grpcmodels.PutChunkDCResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.PutChunkDCRequest{
		Request: convertPutChunkRequestPbToThrift(req.Request),
		NextRVs: req.NextRVs,
	}
	tResp, err := handler.PutChunkDC(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	// Map responses map[string]*PutChunkResponseOrError
	respMap := make(map[string]*grpcmodels.PutChunkResponseOrError, len(tResp.Responses))
	for k, v := range tResp.Responses {
		entry := &grpcmodels.PutChunkResponseOrError{}
		if v.Response != nil {
			entry.Response = &grpcmodels.PutChunkResponse{
				TimeTaken:       v.Response.TimeTaken,
				AvailableSpace:  v.Response.AvailableSpace,
				ComponentRV:     convertRVListThriftToPb(v.Response.ComponentRV),
				ClustermapEpoch: v.Response.ClustermapEpoch,
			}
		} else if v.Error != nil {
			entry.Error = &grpcmodels.ResponseError{
				Code:    grpcmodels.ErrorCode(v.Error.Code),
				Message: v.Error.Message,
			}
		}
		respMap[k] = entry
	}
	return &grpcmodels.PutChunkDCResponse{Responses: respMap, ClustermapEpoch: tResp.ClustermapEpoch}, nil
}
func (g *grpcChunkService) RemoveChunk(ctx context.Context, req *grpcmodels.RemoveChunkRequest) (*grpcmodels.RemoveChunkResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.RemoveChunkRequest{
		SenderNodeID:    req.SenderNodeID,
		Address:         convertAddressPbToThrift(req.Address),
		ComponentRV:     convertRVListPbToThrift(req.ComponentRV),
		ClustermapEpoch: req.ClustermapEpoch,
	}
	tResp, err := handler.RemoveChunk(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.RemoveChunkResponse{
		TimeTaken:        tResp.TimeTaken,
		AvailableSpace:   tResp.AvailableSpace,
		ComponentRV:      convertRVListThriftToPb(tResp.ComponentRV),
		NumChunksDeleted: tResp.NumChunksDeleted,
		ClustermapEpoch:  tResp.ClustermapEpoch,
	}, nil
}
func (g *grpcChunkService) JoinMV(ctx context.Context, req *grpcmodels.JoinMVRequest) (*grpcmodels.JoinMVResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.JoinMVRequest{SenderNodeID: req.SenderNodeID, MV: req.MV, RVName: req.RVName, ReserveSpace: req.ReserveSpace, ComponentRV: convertRVListPbToThrift(req.ComponentRV), ClustermapEpoch: req.ClustermapEpoch}
	tResp, err := handler.JoinMV(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.JoinMVResponse{ClustermapEpoch: tResp.ClustermapEpoch}, nil
}
func (g *grpcChunkService) UpdateMV(ctx context.Context, req *grpcmodels.UpdateMVRequest) (*grpcmodels.UpdateMVResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.UpdateMVRequest{SenderNodeID: req.SenderNodeID, MV: req.MV, RVName: req.RVName, ComponentRV: convertRVListPbToThrift(req.ComponentRV), ClustermapEpoch: req.ClustermapEpoch}
	tResp, err := handler.UpdateMV(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.UpdateMVResponse{ClustermapEpoch: tResp.ClustermapEpoch}, nil
}
func (g *grpcChunkService) LeaveMV(ctx context.Context, req *grpcmodels.LeaveMVRequest) (*grpcmodels.LeaveMVResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.LeaveMVRequest{SenderNodeID: req.SenderNodeID, MV: req.MV, RVName: req.RVName, ComponentRV: convertRVListPbToThrift(req.ComponentRV), ClustermapEpoch: req.ClustermapEpoch}
	tResp, err := handler.LeaveMV(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.LeaveMVResponse{ClustermapEpoch: tResp.ClustermapEpoch}, nil
}
func (g *grpcChunkService) GetMVSize(ctx context.Context, req *grpcmodels.GetMVSizeRequest) (*grpcmodels.GetMVSizeResponse, error) {
	if handler == nil {
		return nil, status.Error(codes.Unavailable, "handler not initialized")
	}
	tReq := &thriftmodels.GetMVSizeRequest{SenderNodeID: req.SenderNodeID, MV: req.MV, RVName: req.RVName, ClustermapEpoch: req.ClustermapEpoch}
	tResp, err := handler.GetMVSize(ctx, tReq)
	if err != nil {
		return nil, mapError(err)
	}
	return &grpcmodels.GetMVSizeResponse{MvSize: tResp.MvSize, ClustermapEpoch: tResp.ClustermapEpoch}, nil
}

// mapError converts internal errors to gRPC status.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

// Placeholder to show where future metrics can be added.
var _ = time.Now

// ----- Conversion helpers -----
func convertAddressPbToThrift(a *grpcmodels.Address) *thriftmodels.Address {
	if a == nil {
		return nil
	}
	return &thriftmodels.Address{FileID: a.FileID, RvID: a.RvID, MvName: a.MvName, OffsetInMiB: a.OffsetInMiB}
}
func convertAddressThriftToPb(a *thriftmodels.Address) *grpcmodels.Address {
	if a == nil {
		return nil
	}
	return &grpcmodels.Address{FileID: a.FileID, RvID: a.RvID, MvName: a.MvName, OffsetInMiB: a.OffsetInMiB}
}
func convertChunkPbToThrift(c *grpcmodels.Chunk) *thriftmodels.Chunk {
	if c == nil {
		return nil
	}
	return &thriftmodels.Chunk{Address: convertAddressPbToThrift(c.Address), Data: c.Data, Hash: c.Hash}
}
func convertChunkThriftToPb(c *thriftmodels.Chunk) *grpcmodels.Chunk {
	if c == nil {
		return nil
	}
	return &grpcmodels.Chunk{Address: convertAddressThriftToPb(c.Address), Data: c.Data, Hash: c.Hash}
}
func convertRVListPbToThrift(lst []*grpcmodels.RVNameAndState) []*thriftmodels.RVNameAndState {
	if len(lst) == 0 {
		return nil
	}
	out := make([]*thriftmodels.RVNameAndState, 0, len(lst))
	for _, v := range lst {
		out = append(out, &thriftmodels.RVNameAndState{Name: v.Name, State: v.State})
	}
	return out
}
func convertRVListThriftToPb(lst []*thriftmodels.RVNameAndState) []*grpcmodels.RVNameAndState {
	if len(lst) == 0 {
		return nil
	}
	out := make([]*grpcmodels.RVNameAndState, 0, len(lst))
	for _, v := range lst {
		out = append(out, &grpcmodels.RVNameAndState{Name: v.Name, State: v.State})
	}
	return out
}
func convertPutChunkRequestPbToThrift(r *grpcmodels.PutChunkRequest) *thriftmodels.PutChunkRequest {
	if r == nil {
		return nil
	}
	return &thriftmodels.PutChunkRequest{SenderNodeID: r.SenderNodeID, Chunk: convertChunkPbToThrift(r.Chunk), Length: r.Length, SyncID: r.SyncID, SourceRVName: r.SourceRVName, ComponentRV: convertRVListPbToThrift(r.ComponentRV), MaybeOverwrite: r.MaybeOverwrite, ClustermapEpoch: r.ClustermapEpoch}
}
