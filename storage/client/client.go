/*
 * Copyright (c) 2021 Gilles Chehade <gilles@poolp.org>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package client

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"github.com/poolpOrg/plakar/network"
	"github.com/poolpOrg/plakar/storage"

	"sync"

	"github.com/poolpOrg/plakar/cache"
)

type ClientRepository struct {
	config storage.RepositoryConfig

	Cache *cache.Cache

	encoder *gob.Encoder
	decoder *gob.Decoder
	mu      sync.Mutex

	Repository string

	inflightRequests map[string]chan network.Request
	//registerInflight     chan inflight
	notifications chan network.Request
	//maxConcurrentRequest chan bool

	storage.RepositoryBackend
}

type ClientTransaction struct {
	Uuid       string
	repository *ClientRepository

	storage.TransactionBackend
}

func init() {
	network.ProtocolRegister()
	storage.Register("client", NewClientRepository)
}

func NewClientRepository() storage.RepositoryBackend {
	return &ClientRepository{}
}

func (repository *ClientRepository) connect(location *url.URL) error {
	scheme := location.Scheme
	switch scheme {
	case "plakar":
		err := repository.connectTCP(location)
		if err != nil {
			return err
		}
	case "ssh":
		err := repository.connectSSH(location)
		if err != nil {
			return err
		}
	case "stdio":
		err := repository.connectStdio(location)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported protocol")
	}

	return nil
}

func (repository *ClientRepository) connectTCP(location *url.URL) error {
	port := location.Port()
	if port == "" {
		port = "9876"
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", location.Hostname()+":"+port)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Fatal(err)
	}

	repository.encoder = gob.NewEncoder(conn)
	repository.decoder = gob.NewDecoder(conn)

	repository.inflightRequests = make(map[string]chan network.Request)
	repository.notifications = make(chan network.Request)

	//repository.maxConcurrentRequest = make(chan bool, 1024)

	go func() {
		for m := range repository.notifications {
			repository.mu.Lock()
			notify := repository.inflightRequests[m.Uuid]
			repository.mu.Unlock()
			notify <- m
		}
	}()

	go func() {
		for {
			result := network.Request{}
			err = repository.decoder.Decode(&result)
			if err != nil {
				conn.Close()
				return
			}
			repository.notifications <- result
		}
	}()

	return err
}

func (repository *ClientRepository) connectStdio(location *url.URL) error {
	subProcess := exec.Command("plakar", "-no-cache", "stdio")

	stdin, err := subProcess.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := subProcess.StdoutPipe()
	if err != nil {
		return err
	}
	subProcess.Stderr = os.Stderr

	repository.encoder = gob.NewEncoder(stdin)
	repository.decoder = gob.NewDecoder(stdout)

	if err = subProcess.Start(); err != nil {
		return err
	}

	repository.inflightRequests = make(map[string]chan network.Request)
	repository.notifications = make(chan network.Request)

	go func() {
		for m := range repository.notifications {
			repository.mu.Lock()
			notify := repository.inflightRequests[m.Uuid]
			repository.mu.Unlock()
			notify <- m
		}
	}()

	go func() {
		for {
			result := network.Request{}
			err = repository.decoder.Decode(&result)
			if err != nil {
				stdin.Close()
				subProcess.Wait()
				return
			}
			repository.notifications <- result
		}
	}()

	return nil
}

func (repository *ClientRepository) connectSSH(location *url.URL) error {
	connectUrl := "ssh://"
	if location.User != nil {
		connectUrl += location.User.Username() + "@"
	}
	connectUrl += location.Hostname()
	if location.Port() != "" {
		connectUrl += ":" + location.Port()
	}

	subProcess := exec.Command("ssh", connectUrl, "plakar -no-cache stdio")

	stdin, err := subProcess.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := subProcess.StdoutPipe()
	if err != nil {
		return err
	}

	subProcess.Stderr = os.Stderr

	repository.encoder = gob.NewEncoder(stdin)
	repository.decoder = gob.NewDecoder(stdout)

	if err = subProcess.Start(); err != nil {
		return err
	}

	repository.inflightRequests = make(map[string]chan network.Request)
	repository.notifications = make(chan network.Request)

	go func() {
		for m := range repository.notifications {
			repository.mu.Lock()
			notify := repository.inflightRequests[m.Uuid]
			repository.mu.Unlock()
			notify <- m
		}
	}()

	go func() {
		for {
			result := network.Request{}
			err = repository.decoder.Decode(&result)
			if err != nil {
				stdin.Close()
				subProcess.Wait()
				return
			}
			repository.notifications <- result
		}
	}()

	return nil
}

func (repository *ClientRepository) sendRequest(Type string, Payload interface{}) (*network.Request, error) {
	Uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	request := network.Request{
		Uuid:    Uuid.String(),
		Type:    Type,
		Payload: Payload,
	}

	notify := make(chan network.Request)
	repository.mu.Lock()
	repository.inflightRequests[request.Uuid] = notify
	repository.mu.Unlock()

	err = repository.encoder.Encode(&request)
	if err != nil {
		return nil, err
	}

	result := <-notify

	repository.mu.Lock()
	delete(repository.inflightRequests, request.Uuid)
	repository.mu.Unlock()
	close(notify)

	return &result, nil
}

func (repository *ClientRepository) Create(location string, config storage.RepositoryConfig) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return err
	}

	err = repository.connect(parsed)
	if err != nil {
		return err
	}

	result, err := repository.sendRequest("ReqCreate", network.ReqCreate{
		Repository:       parsed.Path,
		RepositoryConfig: config,
	})
	if err != nil {
		return err
	}

	if result.Payload.(network.ResCreate).Err != nil {
		return result.Payload.(network.ResCreate).Err
	}

	repository.config = config
	return nil
}

func (repository *ClientRepository) Open(location string) error {
	parsed, err := url.Parse(location)
	if err != nil {
		return err
	}

	err = repository.connect(parsed)
	if err != nil {
		return err
	}

	result, err := repository.sendRequest("ReqOpen", network.ReqOpen{
		Repository: parsed.Path,
	})
	if err != nil {
		return err
	}

	if result.Payload.(network.ResOpen).Err != nil {
		return result.Payload.(network.ResOpen).Err
	}

	repository.config = *result.Payload.(network.ResOpen).RepositoryConfig
	return nil
}

func (repository *ClientRepository) Configuration() storage.RepositoryConfig {
	return repository.config
}

func (repository *ClientRepository) Transaction() (storage.TransactionBackend, error) {
	result, err := repository.sendRequest("ReqTransaction", nil)
	if err != nil {
		return nil, err
	}

	Uuid, err := result.Payload.(network.ResTransaction).Uuid, result.Payload.(network.ResTransaction).Err
	if err != nil {
		return nil, err
	}
	tx := &ClientTransaction{}
	tx.Uuid = Uuid
	tx.repository = repository
	return tx, nil
}

func (repository *ClientRepository) GetIndexes() ([]string, error) {
	result, err := repository.sendRequest("ReqGetIndexes", nil)
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetIndexes).Indexes, result.Payload.(network.ResGetIndexes).Err
}

func (repository *ClientRepository) GetChunks() ([]string, error) {
	result, err := repository.sendRequest("ReqGetChunks", nil)
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetChunks).Chunks, result.Payload.(network.ResGetChunks).Err
}

func (repository *ClientRepository) GetObjects() ([]string, error) {
	result, err := repository.sendRequest("ReqGetObjects", nil)
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetObjects).Objects, result.Payload.(network.ResGetObjects).Err
}

func (repository *ClientRepository) GetMetadata(Uuid string) ([]byte, error) {
	result, err := repository.sendRequest("ReqGetMetadata", network.ReqGetMetadata{
		Uuid: Uuid,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetMetadata).Data, result.Payload.(network.ResGetMetadata).Err
}

func (repository *ClientRepository) GetIndex(Uuid string) ([]byte, error) {
	result, err := repository.sendRequest("ReqGetIndex", network.ReqGetIndex{
		Uuid: Uuid,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetIndex).Data, result.Payload.(network.ResGetIndex).Err
}

func (repository *ClientRepository) GetObject(checksum string) ([]byte, error) {
	result, err := repository.sendRequest("ReqGetObject", network.ReqGetObject{
		Checksum: checksum,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetObject).Data, result.Payload.(network.ResGetObject).Err
}

func (repository *ClientRepository) GetChunk(checksum string) ([]byte, error) {
	result, err := repository.sendRequest("ReqGetChunk", network.ReqGetChunk{
		Checksum: checksum,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResGetChunk).Data, result.Payload.(network.ResGetChunk).Err
}

func (repository *ClientRepository) GetObjectRefCount(checksum string) (uint64, error) {
	result, err := repository.sendRequest("ReqGetObjectRefCount", network.ReqGetObjectRefCount{
		Checksum: checksum,
	})
	if err != nil {
		return 0, err
	}

	return result.Payload.(network.ResGetObjectRefCount).RefCount, result.Payload.(network.ResGetObjectRefCount).Err
}

func (repository *ClientRepository) GetChunkRefCount(checksum string) (uint64, error) {
	result, err := repository.sendRequest("ReqGetChunkRefCount", network.ReqGetChunkRefCount{
		Checksum: checksum,
	})
	if err != nil {
		return 0, err
	}

	return result.Payload.(network.ResGetChunkRefCount).RefCount, result.Payload.(network.ResGetChunkRefCount).Err
}

func (repository *ClientRepository) GetObjectSize(checksum string) (uint64, error) {
	result, err := repository.sendRequest("ReqGetObjectSize", network.ReqGetObjectSize{
		Checksum: checksum,
	})
	if err != nil {
		return 0, err
	}

	return result.Payload.(network.ResGetObjectSize).Size, result.Payload.(network.ResGetObjectSize).Err
}

func (repository *ClientRepository) GetChunkSize(checksum string) (uint64, error) {
	result, err := repository.sendRequest("ReqGetChunkSize", network.ReqGetChunkSize{
		Checksum: checksum,
	})
	if err != nil {
		return 0, err
	}

	return result.Payload.(network.ResGetChunkSize).Size, result.Payload.(network.ResGetChunkSize).Err
}

func (repository *ClientRepository) Purge(id string) error {
	result, err := repository.sendRequest("ReqPurge", network.ReqPurge{
		Uuid: id,
	})
	if err != nil {
		return err
	}

	return result.Payload.(network.ResPurge).Err
}

func (repository *ClientRepository) Close() error {
	result, err := repository.sendRequest("ReqClose", nil)
	if err != nil {
		return err
	}

	return result.Payload.(network.ResClose).Err
}

//////

func (transaction *ClientTransaction) GetUuid() string {
	return transaction.Uuid
}
func (transaction *ClientTransaction) ReferenceChunks(keys []string) ([]bool, error) {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqReferenceChunks", network.ReqReferenceChunks{
		Transaction: transaction.GetUuid(),
		Keys:        keys,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResReferenceChunks).Exists, result.Payload.(network.ResReferenceChunks).Err
}

func (transaction *ClientTransaction) ReferenceObjects(keys []string) ([]bool, error) {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqReferenceObjects", network.ReqReferenceObjects{
		Transaction: transaction.GetUuid(),
		Keys:        keys,
	})
	if err != nil {
		return nil, err
	}

	return result.Payload.(network.ResReferenceObjects).Exists, result.Payload.(network.ResReferenceObjects).Err
}

func (transaction *ClientTransaction) PutObject(checksum string, data []byte) error {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqPutObject", network.ReqPutObject{
		Transaction: transaction.GetUuid(),
		Checksum:    checksum,
		Data:        data,
	})
	if err != nil {
		return err
	}

	return result.Payload.(network.ResPutObject).Err
}

func (transaction *ClientTransaction) PutChunk(checksum string, data []byte) error {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqPutChunk", network.ReqPutChunk{
		Transaction: transaction.GetUuid(),
		Checksum:    checksum,
		Data:        data,
	})
	if err != nil {
		return err
	}
	return result.Payload.(network.ResPutChunk).Err
}

func (transaction *ClientTransaction) PutMetadata(data []byte) error {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqPutMetadata", network.ReqPutMetadata{
		Transaction: transaction.GetUuid(),
		Data:        data,
	})
	if err != nil {
		return err
	}

	return result.Payload.(network.ResPutMetadata).Err
}

func (transaction *ClientTransaction) PutIndex(data []byte) error {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqPutIndex", network.ReqPutIndex{
		Transaction: transaction.GetUuid(),
		Data:        data,
	})
	if err != nil {
		return err
	}

	return result.Payload.(network.ResPutIndex).Err
}

func (transaction *ClientTransaction) Commit() error {
	repository := transaction.repository
	result, err := repository.sendRequest("ReqCommit", network.ReqCommit{
		Transaction: transaction.GetUuid(),
	})
	if err != nil {
		return err
	}
	return result.Payload.(network.ResCommit).Err
}
