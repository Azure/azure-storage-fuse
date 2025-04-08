package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Azure/azure-storage-fuse/v2/common"
	"github.com/Azure/azure-storage-fuse/v2/common/log"
	"github.com/Azure/azure-storage-fuse/v2/internal/dcache_lib/rpc/gen-go/dcache"
)

// type check to ensure that ChunkServiceHandler implements dcache.ChunkService interface
var _ dcache.ChunkService = &ChunkServiceHandler{}

// ChunkServiceHandler struct implements the ChunkService interface
type ChunkServiceHandler struct {
	cacheDir string
	locks    *common.LockMap
}

func NewChunkServiceHandler(cacheDir string) *ChunkServiceHandler {
	return &ChunkServiceHandler{
		locks:    common.NewLockMap(),
		cacheDir: cacheDir,
	}
}

func (h *ChunkServiceHandler) getChunkDataPath(mirrorVolume int64, fileID string, offset int64) string {
	return filepath.Join(h.cacheDir, fmt.Sprintf("mv%v/%v.%v.data", mirrorVolume, fileID, offset))
}

func (h *ChunkServiceHandler) getChunkHashPath(mirrorVolume int64, fileID string, offset int64) string {
	return filepath.Join(h.cacheDir, fmt.Sprintf("mv%v/%v.%v.sha1", mirrorVolume, fileID, offset))
}

func (h *ChunkServiceHandler) Ping(ctx context.Context) error {
	log.Debug("ChunkServiceHandler::Ping: request received")
	return nil
}

func (h *ChunkServiceHandler) GetChunk(ctx context.Context, fileID string, fsID string, mirrorVolume int64, offset int64) (*dcache.Chunk, error) {
	log.Debug("ChunkServiceHandler::GetChunk: request received for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d", fileID, fsID, mirrorVolume, offset)

	chunkDataPath := h.getChunkDataPath(mirrorVolume, fileID, offset)
	chunkHashPath := h.getChunkHashPath(mirrorVolume, fileID, offset)

	// check if the chunk file is being written to in parallel by some other thread
	isLocked := h.locks.Locked(chunkDataPath)
	if isLocked {
		log.Err("ChunkServiceHandler::GetChunk: chunk %v is being written", chunkDataPath)
		return nil, fmt.Errorf("chunk %v is being written", chunkDataPath)
	}

	data, err := os.ReadFile(chunkDataPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to get chunk data %v [%v]", chunkDataPath, err.Error())
		return nil, err
	}

	hash, err := os.ReadFile(chunkHashPath)
	if err != nil {
		log.Err("ChunkServiceHandler::GetChunk: Failed to get chunk hash %v [%v]", chunkHashPath, err.Error())
		return nil, err
	}

	chunk := &dcache.Chunk{
		FileID:       fileID,
		FsID:         fsID,
		MirrorVolume: mirrorVolume,
		Offset:       offset,
		Length:       int64(len(data)),
		Hash:         string(hash),
		Data:         data,
	}
	return chunk, nil
}

func (h *ChunkServiceHandler) PutChunk(ctx context.Context, chunk *dcache.Chunk) error {
	log.Debug("ChunkServiceHandler::PutChunk: request received for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d, hash: %s", chunk.FileID, chunk.FsID, chunk.MirrorVolume, chunk.Offset, chunk.Hash)

	chunkDataPath := h.getChunkDataPath(chunk.MirrorVolume, chunk.FileID, chunk.Offset)
	chunkHashPath := h.getChunkHashPath(chunk.MirrorVolume, chunk.FileID, chunk.Offset)

	// acquire lock so that no other thread can write to this chunk
	flock := h.locks.Get(chunkDataPath)
	flock.Lock()
	defer flock.Unlock()

	// check if the chunk file already exists
	if _, err := os.Stat(chunkDataPath); err == nil {
		log.Err("ChunkServiceHandler::PutChunk: chunk %v already exists", chunkDataPath)
		return fmt.Errorf("chunk %v already exists", chunkDataPath)
	}
	// Create the directory if it doesn't exist
	dirPath := filepath.Dir(chunkDataPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to create directory %v [%v]", dirPath, err.Error())
		return err
	}

	err := os.WriteFile(chunkDataPath, chunk.Data, 0400)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to write chunk data %v [%v]", chunkDataPath, err.Error())
		return err
	}

	err = os.WriteFile(chunkHashPath, []byte(chunk.Hash), 0400)
	if err != nil {
		log.Err("ChunkServiceHandler::PutChunk: Failed to write chunk hash %v [%v]", chunkHashPath, err.Error())
		return err
	}

	// TODO: should we verify the hash after writing
	return nil
}

func (h *ChunkServiceHandler) RemoveChunk(ctx context.Context, fileID string, fsID string, mirrorVolume int64, offset int64) error {
	log.Debug("ChunkServiceHandler::RemoveChunk: request received for fileID: %s, fsID: %s, mirrorVolume: %d, offset: %d", fileID, fsID, mirrorVolume, offset)

	chunkDataPath := h.getChunkDataPath(mirrorVolume, fileID, offset)
	chunkHashPath := h.getChunkHashPath(mirrorVolume, fileID, offset)

	// check if the chunk file is being written to in parallel by some other thread
	isLocked := h.locks.Locked(chunkDataPath)
	if isLocked {
		log.Err("ChunkServiceHandler::RemoveChunk: chunk %v is being written", chunkDataPath)
		return fmt.Errorf("chunk %v is being written", chunkDataPath)
	}

	err := os.Remove(chunkDataPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove chunk data %v [%v]", chunkDataPath, err.Error())
	}

	err = os.Remove(chunkHashPath)
	if err != nil {
		log.Err("ChunkServiceHandler::RemoveChunk: Failed to remove chunk hash %v [%v]", chunkHashPath, err.Error())
	}

	// delete the lock item from the map
	h.locks.Delete(chunkDataPath)

	return nil
}
