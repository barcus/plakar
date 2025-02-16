package network

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"github.com/poolpOrg/plakar/logger"
	"github.com/poolpOrg/plakar/storage"
)

func Server(repository *storage.Repository, addr string) {

	ProtocolRegister()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection(c, c)
	}
}

func Stdio() error {
	ProtocolRegister()
	handleConnection(os.Stdin, os.Stdout)
	return nil
}

func handleConnection(rd io.Reader, wr io.Writer) {
	decoder := gob.NewDecoder(rd)
	encoder := gob.NewEncoder(wr)

	transactions := make(map[string]*storage.Transaction)

	var repository *storage.Repository
	var wg sync.WaitGroup
	Uuid, _ := uuid.NewRandom()
	clientUuid := Uuid.String()

	homeDir := os.Getenv("HOME")

	for {
		request := Request{}
		err := decoder.Decode(&request)
		if err != nil {
			break
		}

		switch request.Type {
		case "ReqCreate":
			wg.Add(1)
			go func() {
				defer wg.Done()

				dirPath := request.Payload.(ReqCreate).Repository
				if dirPath == "" {
					dirPath = filepath.Join(homeDir, ".plakar")
				}

				logger.Trace("%s: Create(%s, %s)", clientUuid, dirPath, request.Payload.(ReqCreate).RepositoryConfig)
				repository, err = storage.Create(dirPath, request.Payload.(ReqCreate).RepositoryConfig)
				result := Request{
					Uuid:    request.Uuid,
					Type:    "ResCreate",
					Payload: ResCreate{Err: err},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqOpen":
			wg.Add(1)
			go func() {
				defer wg.Done()

				dirPath := request.Payload.(ReqOpen).Repository
				if dirPath == "" {
					dirPath = filepath.Join(homeDir, ".plakar")
				}

				logger.Trace("%s: Open(%s)", clientUuid, dirPath)
				repository, err = storage.Open(dirPath)
				var payload ResOpen
				if err != nil {
					payload = ResOpen{RepositoryConfig: nil, Err: err}
				} else {
					config := repository.Configuration()
					payload = ResOpen{RepositoryConfig: &config, Err: nil}
				}
				result := Request{
					Uuid:    request.Uuid,
					Type:    "ResOpen",
					Payload: payload,
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetIndexes":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetIndexes", clientUuid)
				indexes, err := repository.GetIndexes()
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetIndexes",
					Payload: ResGetIndexes{
						Indexes: indexes,
						Err:     err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetChunks":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetChunks", clientUuid)
				chunks, err := repository.GetChunks()
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetChunks",
					Payload: ResGetChunks{
						Chunks: chunks,
						Err:    err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetObjects":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetObjects", clientUuid)
				objects, err := repository.GetObjects()
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetObjects",
					Payload: ResGetObjects{
						Objects: objects,
						Err:     err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetMetadata":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetMetadata(%s)", clientUuid, request.Payload.(ReqGetMetadata).Uuid)
				data, err := repository.GetMetadata(request.Payload.(ReqGetMetadata).Uuid)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetMetadata",
					Payload: ResGetMetadata{
						Data: data,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetIndex":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetIndex(%s)", clientUuid, request.Payload.(ReqGetIndex).Uuid)
				data, err := repository.GetIndex(request.Payload.(ReqGetIndex).Uuid)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetIndex",
					Payload: ResGetIndex{
						Data: data,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetObject":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetObject(%s)", clientUuid, request.Payload.(ReqGetObject).Checksum)
				data, err := repository.GetObject(request.Payload.(ReqGetObject).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetObject",
					Payload: ResGetObject{
						Data: data,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetChunk":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: GetChunk(%s)", clientUuid, request.Payload.(ReqGetChunk).Checksum)
				data, err := repository.GetChunk(request.Payload.(ReqGetChunk).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetChunk",
					Payload: ResGetChunk{
						Data: data,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqCheckObject":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: CheckObject(%s)", clientUuid, request.Payload.(ReqCheckObject).Checksum)
				exists, err := repository.CheckObject(request.Payload.(ReqCheckObject).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResCheckObject",
					Payload: ResCheckObject{
						Exists: exists,
						Err:    err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqCheckChunk":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: CheckChunk(%s)", clientUuid, request.Payload.(ReqCheckChunk).Checksum)
				exists, err := repository.CheckChunk(request.Payload.(ReqCheckChunk).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResCheckChunk",
					Payload: ResCheckChunk{
						Exists: exists,
						Err:    err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetChunkRefCount":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetChunkRefCount(%s)", clientUuid, request.Payload.(ReqGetChunkRefCount).Checksum)
				refCount, err := repository.GetChunkRefCount(request.Payload.(ReqGetChunkRefCount).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetChunkRefCount",
					Payload: ResGetChunkRefCount{
						RefCount: refCount,
						Err:      err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetObjectRefCount":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetObjectRefCount(%s)", clientUuid, request.Payload.(ReqGetObjectRefCount).Checksum)
				refCount, err := repository.GetObjectRefCount(request.Payload.(ReqGetObjectRefCount).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetObjectRefCount",
					Payload: ResGetObjectRefCount{
						RefCount: refCount,
						Err:      err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetChunkSize":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetChunkSize(%s)", clientUuid, request.Payload.(ReqGetChunkSize).Checksum)
				size, err := repository.GetChunkSize(request.Payload.(ReqGetChunkSize).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetChunkSize",
					Payload: ResGetChunkSize{
						Size: size,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqGetObjectSize":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: GetObjectSize(%s)", clientUuid, request.Payload.(ReqGetObjectSize).Checksum)
				size, err := repository.GetObjectSize(request.Payload.(ReqGetObjectSize).Checksum)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResGetObjectSize",
					Payload: ResGetObjectSize{
						Size: size,
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqPurge":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: Purge(%s)", clientUuid, request.Payload.(ReqPurge).Uuid)
				err := repository.Purge(request.Payload.(ReqPurge).Uuid)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResPurge",
					Payload: ResPurge{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqTransaction":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: Transaction", clientUuid)
				tx, err := repository.Transaction()
				result := Request{
					Uuid: request.Uuid,
					Type: "ResTransaction",
					Payload: ResTransaction{
						Uuid: tx.GetUuid(),
						Err:  err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
				transactions[tx.GetUuid()] = tx
			}()

		case "ReqReferenceChunks":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: ReferenceChunks()", clientUuid)
				txUuid := request.Payload.(ReqReferenceChunks).Transaction
				tx := transactions[txUuid]
				exists, err := tx.ReferenceChunks(request.Payload.(ReqReferenceChunks).Keys)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResReferenceChunks",
					Payload: ResReferenceChunks{
						Exists: exists,
						Err:    err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqReferenceObjects":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: ReferenceObjects()", clientUuid)
				txUuid := request.Payload.(ReqReferenceObjects).Transaction
				tx := transactions[txUuid]
				exists, err := tx.ReferenceObjects(request.Payload.(ReqReferenceObjects).Keys)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResReferenceObjects",
					Payload: ResReferenceObjects{
						Exists: exists,
						Err:    err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqPutChunk":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: PutChunk(%s)", clientUuid, request.Payload.(ReqPutChunk).Checksum)
				txUuid := request.Payload.(ReqPutChunk).Transaction
				tx := transactions[txUuid]
				err := tx.PutChunk(request.Payload.(ReqPutChunk).Checksum, request.Payload.(ReqPutChunk).Data)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResPutChunk",
					Payload: ResPutChunk{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqPutObject":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: PutObject(%s)", clientUuid, request.Payload.(ReqPutObject).Checksum)
				txUuid := request.Payload.(ReqPutObject).Transaction
				tx := transactions[txUuid]
				err := tx.PutObject(request.Payload.(ReqPutObject).Checksum, request.Payload.(ReqPutObject).Data)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResPutObject",
					Payload: ResPutObject{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqPutMetadata":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: PutMetadata()", clientUuid)
				txUuid := request.Payload.(ReqPutMetadata).Transaction
				tx := transactions[txUuid]
				err := tx.PutMetadata(request.Payload.(ReqPutMetadata).Data)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResPutMetadata",
					Payload: ResPutMetadata{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqPutIndex":
			wg.Add(1)
			go func() {
				defer wg.Done()
				logger.Trace("%s: PutIndex()", clientUuid)
				txUuid := request.Payload.(ReqPutIndex).Transaction
				tx := transactions[txUuid]
				err := tx.PutIndex(request.Payload.(ReqPutIndex).Data)
				result := Request{
					Uuid: request.Uuid,
					Type: "ResPutIndex",
					Payload: ResPutIndex{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqCommit":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: Commit()", clientUuid)
				txUuid := request.Payload.(ReqCommit).Transaction
				tx := transactions[txUuid]
				err := tx.Commit()
				result := Request{
					Uuid: request.Uuid,
					Type: "ResCommit",
					Payload: ResCommit{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()

		case "ReqClose":
			wg.Add(1)
			go func() {
				defer wg.Done()

				logger.Trace("%s: Close()", clientUuid)
				err := repository.Close()
				repository = nil
				result := Request{
					Uuid: request.Uuid,
					Type: "ResClose",
					Payload: ResClose{
						Err: err,
					},
				}
				err = encoder.Encode(&result)
				if err != nil {
					logger.Warn("%s", err)
				}
			}()
		}
	}
	wg.Wait()

	if repository != nil {
		repository.Close()
	}
}
