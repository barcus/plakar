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

package fs

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/poolpOrg/plakar/cache"
	"github.com/poolpOrg/plakar/compression"
	"github.com/poolpOrg/plakar/logger"
	"github.com/poolpOrg/plakar/storage"

	"github.com/google/uuid"
	"github.com/iafan/cwalk"
)

type FSRepository struct {
	config storage.RepositoryConfig

	Cache *cache.Cache

	Repository string
	root       string
	dirty      bool

	storage.RepositoryBackend
}

type FSTransaction struct {
	Uuid       string
	repository FSRepository
	prepared   bool

	//SkipDirs []string

	chunksMutex  sync.Mutex
	objectsMutex sync.Mutex

	chunks  map[string]bool
	objects map[string]bool

	muChunkBucket sync.Mutex
	chunkBucket   map[string]bool

	muObjectBucket sync.Mutex
	objectBucket   map[string]bool

	storage.TransactionBackend
}

func init() {
	storage.Register("filesystem", NewFSRepository)
}

func NewFSRepository() storage.RepositoryBackend {
	return &FSRepository{}
}

func (repository *FSRepository) objectExists(checksum string) bool {
	return pathnameExists(repository.PathObject(checksum))
}

func (repository *FSRepository) chunkExists(checksum string) bool {
	return pathnameExists(repository.PathChunk(checksum))
}

func (repository *FSRepository) Create(location string, config storage.RepositoryConfig) error {
	t0 := time.Now()
	defer func() {
		logger.Profile("Create(%s): %s", location, time.Since(t0))
	}()

	repository.root = location

	err := os.Mkdir(repository.root, 0700)
	if err != nil {
		return err
	}
	os.MkdirAll(fmt.Sprintf("%s/chunks", repository.root), 0700)
	os.MkdirAll(fmt.Sprintf("%s/objects", repository.root), 0700)
	os.MkdirAll(fmt.Sprintf("%s/transactions", repository.root), 0700)
	os.MkdirAll(fmt.Sprintf("%s/snapshots", repository.root), 0700)
	os.MkdirAll(fmt.Sprintf("%s/purge", repository.root), 0700)

	for i := 0; i < 256; i++ {
		os.MkdirAll(fmt.Sprintf("%s/chunks/%02x", repository.root, i), 0700)
		os.MkdirAll(fmt.Sprintf("%s/objects/%02x", repository.root, i), 0700)
		os.MkdirAll(fmt.Sprintf("%s/transactions/%02x", repository.root, i), 0700)
		os.MkdirAll(fmt.Sprintf("%s/snapshots/%02x", repository.root, i), 0700)
	}

	f, err := os.Create(fmt.Sprintf("%s/CONFIG", repository.root))
	if err != nil {
		return err
	}
	defer f.Close()

	jconfig, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = f.Write(compression.Deflate(jconfig))
	if err != nil {
		return err
	}

	repository.config = config

	return nil
}

func (repository *FSRepository) Open(location string) error {
	repository.root = location

	compressed, err := ioutil.ReadFile(fmt.Sprintf("%s/CONFIG", repository.root))
	if err != nil {
		return err
	}

	jconfig, err := compression.Inflate(compressed)
	if err != nil {
		return err
	}

	config := storage.RepositoryConfig{}
	err = json.Unmarshal(jconfig, &config)
	if err != nil {
		return err
	}

	repository.config = config

	return nil
}

func (repository *FSRepository) Configuration() storage.RepositoryConfig {
	return repository.config
}

func (repository *FSRepository) Transaction() (storage.TransactionBackend, error) {
	// XXX - keep a map of current transactions

	tx := &FSTransaction{}
	tx.Uuid = uuid.New().String()
	tx.repository = *repository
	tx.prepared = false

	tx.chunks = make(map[string]bool)
	tx.objects = make(map[string]bool)
	tx.chunkBucket = make(map[string]bool)
	tx.objectBucket = make(map[string]bool)

	tx.prepare()

	return tx, nil
}

func (repository *FSRepository) GetIndexes() ([]string, error) {
	ret := make([]string, 0)

	buckets, err := ioutil.ReadDir(repository.PathIndexes())
	if err != nil {
		return ret, nil
	}

	for _, bucket := range buckets {
		indexes, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", repository.PathIndexes(), bucket.Name()))
		if err != nil {
			return ret, err
		}
		for _, index := range indexes {
			_, err = uuid.Parse(index.Name())
			if err != nil {
				return ret, nil
			}
			ret = append(ret, index.Name())
		}
	}
	return ret, nil
}

func (repository *FSRepository) GetMetadata(Uuid string) ([]byte, error) {
	parsedUuid, err := uuid.Parse(Uuid)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/METADATA", repository.PathIndex(parsedUuid.String())))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) GetIndex(Uuid string) ([]byte, error) {
	parsedUuid, err := uuid.Parse(Uuid)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(fmt.Sprintf("%s/INDEX", repository.PathIndex(parsedUuid.String())))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) PutMetadata(id string, data []byte) error {
	os.Mkdir(repository.PathIndexBucket(id), 0700)
	os.Mkdir(repository.PathIndex(id), 0700)
	os.Mkdir(repository.PathIndexObjects(id), 0700)
	os.Mkdir(repository.PathIndexChunks(id), 0700)

	f, err := os.Create(fmt.Sprintf("%s/METADATA", repository.PathIndex(id)))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (repository *FSRepository) PutIndex(id string, data []byte) error {
	os.Mkdir(repository.PathIndexBucket(id), 0700)
	os.Mkdir(repository.PathIndex(id), 0700)
	os.Mkdir(repository.PathIndexObjects(id), 0700)
	os.Mkdir(repository.PathIndexChunks(id), 0700)

	f, err := os.Create(fmt.Sprintf("%s/INDEX", repository.PathIndex(id)))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (repository *FSRepository) ReferenceIndexChunk(id string, checksum string) error {
	os.Mkdir(repository.PathIndexChunkBucket(id, checksum), 0700)
	os.Link(repository.PathChunk(checksum), repository.PathIndexChunk(id, checksum))
	return nil
}

func (repository *FSRepository) ReferenceIndexObject(id string, checksum string) error {
	os.Mkdir(repository.PathIndexObjectBucket(id, checksum), 0700)
	os.Link(repository.PathObject(checksum), repository.PathIndexObject(id, checksum))
	return nil
}

func (repository *FSRepository) GetObjects() ([]string, error) {
	ret := make([]string, 0)

	buckets, err := ioutil.ReadDir(repository.PathObjects())
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		objects, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", repository.PathObjects(), bucket.Name()))
		if err != nil {
			return ret, err
		}

		for _, object := range objects {
			//_, err = uuid.Parse(object.Name())
			//if err != nil {
			//		return ret, nil
			//	}
			ret = append(ret, object.Name())
		}
	}
	return ret, nil
}

func (repository *FSRepository) GetIndexObject(id string, checksum string) ([]byte, error) {
	data, err := ioutil.ReadFile(repository.PathIndexObject(id, checksum))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) GetIndexChunk(id string, checksum string) ([]byte, error) {
	data, err := ioutil.ReadFile(repository.PathIndexChunk(id, checksum))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) GetObject(checksum string) ([]byte, error) {
	data, err := ioutil.ReadFile(repository.PathObject(checksum))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) GetObjectRefCount(checksum string) (uint64, error) {
	st, err := os.Stat(repository.PathObject(checksum))
	if err != nil {
		return 0, err
	}
	return uint64(st.Sys().(*syscall.Stat_t).Nlink - 1), nil
}

func (repository *FSRepository) GetChunkRefCount(checksum string) (uint64, error) {
	st, err := os.Stat(repository.PathChunk(checksum))
	if err != nil {
		return 0, err
	}
	return uint64(st.Sys().(*syscall.Stat_t).Nlink - 1), nil
}

func (repository *FSRepository) GetObjectSize(checksum string) (uint64, error) {
	st, err := os.Stat(repository.PathObject(checksum))
	if err != nil {
		return 0, err
	}
	return uint64(st.Size()), nil
}

func (repository *FSRepository) GetChunkSize(checksum string) (uint64, error) {
	st, err := os.Stat(repository.PathChunk(checksum))
	if err != nil {
		return 0, err
	}
	return uint64(st.Size()), nil
}

func (repository *FSRepository) PutObject(checksum string, data []byte) error {
	f, err := ioutil.TempFile(repository.PathObjectBucket(checksum), fmt.Sprintf("%s.*", checksum))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	err = os.Rename(f.Name(), repository.PathObject(checksum))
	if err != nil {
		return err
	}

	return nil
}

func (repository *FSRepository) PutObjectSafe(checksum string, data []byte, link string) error {
	f, err := ioutil.TempFile(repository.PathObjectBucket(checksum), fmt.Sprintf("%s.*", checksum))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	err = os.Link(f.Name(), link)
	if err != nil {
		return err
	}

	err = os.Rename(f.Name(), repository.PathObject(checksum))
	if err != nil {
		return err
	}

	return nil
}

func (repository *FSRepository) GetChunks() ([]string, error) {
	ret := make([]string, 0)

	buckets, err := ioutil.ReadDir(repository.PathChunks())
	if err != nil {
		return nil, err
	}

	for _, bucket := range buckets {
		chunks, err := ioutil.ReadDir(fmt.Sprintf("%s/%s", repository.PathChunks(), bucket.Name()))
		if err != nil {
			return ret, err
		}

		for _, chunk := range chunks {
			//_, err = uuid.Parse(object.Name())
			//if err != nil {
			//		return ret, nil
			//	}
			ret = append(ret, chunk.Name())
		}
	}
	return ret, nil
}

func (repository *FSRepository) GetChunk(checksum string) ([]byte, error) {
	data, err := ioutil.ReadFile(repository.PathChunk(checksum))
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (repository *FSRepository) PutChunk(checksum string, data []byte) error {
	f, err := ioutil.TempFile(repository.PathChunkBucket(checksum), fmt.Sprintf("%s.*", checksum))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	err = os.Rename(f.Name(), repository.PathChunk(checksum))
	if err != nil {
		return err
	}

	return nil
}

func (repository *FSRepository) PutChunkSafe(checksum string, data []byte, link string) error {
	f, err := ioutil.TempFile(repository.PathChunkBucket(checksum), fmt.Sprintf("%s.*", checksum))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	err = os.Link(f.Name(), link)
	if err != nil {
		return err
	}

	err = os.Rename(f.Name(), repository.PathChunk(checksum))
	if err != nil {
		return err
	}

	return nil
}

func (repository *FSRepository) CheckIndexObject(id string, checksum string) (bool, error) {
	fileinfo, err := os.Stat(repository.PathIndexObject(id, checksum))
	if os.IsNotExist(err) {
		return false, nil
	}
	return fileinfo.Mode().IsRegular(), nil
}

func (repository *FSRepository) CheckIndexChunk(id string, checksum string) (bool, error) {
	fileinfo, err := os.Stat(repository.PathIndexChunk(id, checksum))
	if os.IsNotExist(err) {
		return false, nil
	}
	return fileinfo.Mode().IsRegular(), nil

}

func (repository *FSRepository) CheckObject(checksum string) (bool, error) {
	fileinfo, err := os.Stat(repository.PathObject(checksum))
	if os.IsNotExist(err) {
		return false, nil
	}
	return fileinfo.Mode().IsRegular(), nil
}

func (repository *FSRepository) CheckChunk(checksum string) (bool, error) {
	fileinfo, err := os.Stat(repository.PathChunk(checksum))
	if os.IsNotExist(err) {
		return false, nil
	}
	return fileinfo.Mode().IsRegular(), nil

}

func (repository *FSRepository) Purge(id string) error {
	dest := fmt.Sprintf("%s/%s", repository.PathPurge(), id)
	err := os.Rename(repository.PathIndex(id), dest)
	if err != nil {
		return err
	}

	err = os.RemoveAll(dest)
	if err != nil {
		return err
	}

	repository.dirty = true

	return nil
}

func (repository *FSRepository) Close() error {
	// XXX - rollback all pending transactions so they don't linger
	if repository.dirty {
		repository.Tidy()
		repository.dirty = false
	}
	return nil
}

func (repository *FSRepository) Tidy() {
	wg := sync.WaitGroup{}
	concurrency := make(chan bool, runtime.NumCPU()*2+1)
	cwalk.Walk(repository.PathObjects(), func(path string, f os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		object := fmt.Sprintf("%s/%s", repository.PathObjects(), path)
		if filepath.Clean(object) == filepath.Clean(repository.PathObjects()) {
			return nil
		}
		if !f.IsDir() {
			concurrency <- true
			wg.Add(1)
			go func(object string) {
				defer func() { <-concurrency }()
				defer func() { wg.Done() }()
				if f.Sys().(*syscall.Stat_t).Nlink == 1 {
					os.Remove(object)
				}
			}(object)
		}
		return nil
	})
	wg.Wait()

	cwalk.Walk(repository.PathChunks(), func(path string, f os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		chunk := fmt.Sprintf("%s/%s", repository.PathChunks(), path)
		if filepath.Clean(chunk) == filepath.Clean(repository.PathChunks()) {
			return nil
		}

		if !f.IsDir() {
			concurrency <- true
			wg.Add(1)
			go func(chunk string) {
				defer func() { <-concurrency }()
				defer func() { wg.Done() }()
				if f.Sys().(*syscall.Stat_t).Nlink == 1 {
					os.Remove(chunk)
				}
			}(chunk)
		}
		return nil
	})
	wg.Wait()
}

func (transaction *FSTransaction) CreateChunkBucket(checksum string) error {
	transaction.muChunkBucket.Lock()
	defer transaction.muChunkBucket.Unlock()

	bucket := checksum[0:2]
	if _, exists := transaction.chunkBucket[bucket]; !exists {
		err := os.Mkdir(transaction.PathChunkBucket(checksum), 0700)
		if err != nil {
			return err
		}
		transaction.chunkBucket[bucket] = true
	}
	return nil
}

func (transaction *FSTransaction) CreateObjectBucket(checksum string) error {
	transaction.muObjectBucket.Lock()
	defer transaction.muObjectBucket.Unlock()

	bucket := checksum[0:2]
	if _, exists := transaction.objectBucket[bucket]; !exists {
		err := os.Mkdir(transaction.PathObjectBucket(checksum), 0700)
		if err != nil {
			return err
		}
		transaction.objectBucket[bucket] = true
	}
	return nil
}

func (transaction *FSTransaction) GetUuid() string {
	return transaction.Uuid
}

func (transaction *FSTransaction) prepare() {
	os.MkdirAll(transaction.repository.root, 0700)
	os.MkdirAll(fmt.Sprintf("%s/%s", transaction.repository.PathTransactions(),
		transaction.Uuid[0:2]), 0700)
	os.MkdirAll(transaction.Path(), 0700)
	os.MkdirAll(fmt.Sprintf("%s/chunks", transaction.Path()), 0700)
	os.MkdirAll(fmt.Sprintf("%s/objects", transaction.Path()), 0700)
}

func (transaction *FSTransaction) ReferenceChunks(keys []string) ([]bool, error) {
	ret := make([]bool, 0)
	for _, key := range keys {
		err := transaction.CreateChunkBucket(key)
		if err != nil {
			return nil, err
		}
		err = os.Link(transaction.repository.PathChunk(key), transaction.PathChunk(key))
		if err != nil {
			if os.IsNotExist(err) {
				ret = append(ret, false)
			} else {
				ret = append(ret, true)
				transaction.chunksMutex.Lock()
				transaction.chunks[key] = true
				transaction.chunksMutex.Unlock()
			}
		} else {
			ret = append(ret, true)
			transaction.chunksMutex.Lock()
			transaction.chunks[key] = true
			transaction.chunksMutex.Unlock()
		}
	}

	return ret, nil
}

func (transaction *FSTransaction) ReferenceObjects(keys []string) ([]bool, error) {
	ret := make([]bool, 0)
	for _, key := range keys {
		err := transaction.CreateObjectBucket(key)
		if err != nil {
			return nil, err
		}
		err = os.Link(transaction.repository.PathObject(key), transaction.PathObject(key))
		if err != nil {
			if os.IsNotExist(err) {
				ret = append(ret, false)
			} else {
				ret = append(ret, true)
				transaction.objectsMutex.Lock()
				transaction.objects[key] = true
				transaction.objectsMutex.Unlock()
			}
		} else {
			ret = append(ret, true)
			transaction.objectsMutex.Lock()
			transaction.objects[key] = true
			transaction.objectsMutex.Unlock()
		}
	}

	return ret, nil
}

func (transaction *FSTransaction) PutObject(checksum string, data []byte) error {
	repository := transaction.repository

	err := transaction.CreateObjectBucket(checksum)
	if err != nil {
		return err
	}

	err = repository.PutObjectSafe(checksum, data, transaction.PathObject(checksum))
	if err != nil {
		return err
	}

	transaction.objectsMutex.Lock()
	transaction.objects[checksum] = true
	transaction.repository.dirty = true
	transaction.objectsMutex.Unlock()
	return nil
}

func (transaction *FSTransaction) PutChunk(checksum string, data []byte) error {
	repository := transaction.repository

	err := transaction.CreateChunkBucket(checksum)
	if err != nil {
		return err
	}

	err = repository.PutChunkSafe(checksum, data, transaction.PathChunk(checksum))
	if err != nil {
		return err
	}

	transaction.chunksMutex.Lock()
	transaction.chunks[checksum] = true
	transaction.repository.dirty = true
	transaction.chunksMutex.Unlock()
	return nil
}

func (transaction *FSTransaction) PutMetadata(data []byte) error {
	f, err := os.Create(fmt.Sprintf("%s/METADATA", transaction.Path()))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (transaction *FSTransaction) PutIndex(data []byte) error {
	f, err := os.Create(fmt.Sprintf("%s/INDEX", transaction.Path()))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func (transaction *FSTransaction) Commit() error {
	transaction.repository.dirty = false
	return os.Rename(transaction.Path(), transaction.repository.PathIndex(transaction.Uuid))
}
