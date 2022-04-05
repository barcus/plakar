package network

import (
	"encoding/gob"

	"github.com/poolpOrg/plakar/storage"
)

type Request struct {
	Uuid    string
	Type    string
	Payload interface{}
}

type ReqChallenge struct {
	PublicKey []byte
}

type ResChallenge struct {
	Challenge []byte
}

type ReqChallengeResponse struct {
	Signature []byte
}

type ResChallengeResponse struct {
	Authenticated bool
}

type ReqCreate struct {
	StoreConfig storage.StoreConfig
}

type ResCreate struct {
	Err error
}

type ReqOpen struct {
	Uuid string
}

type ResOpen struct {
	StoreConfig storage.StoreConfig
	Err         error
}

type ReqGetChunks struct {
}

type ResGetChunks struct {
	Chunks []string
	Err    error
}

type ReqGetObjects struct {
}

type ResGetObjects struct {
	Objects []string
	Err     error
}

type ReqGetIndexes struct {
}

type ResGetIndexes struct {
	Indexes []string
	Err     error
}

type ReqGetIndex struct {
	Uuid string
}

type ResGetIndex struct {
	Data []byte
	Err  error
}

type ReqGetObject struct {
	Checksum string
}

type ResGetObject struct {
	Data []byte
	Err  error
}

type ReqGetChunk struct {
	Checksum string
}

type ResGetChunk struct {
	Data []byte
	Err  error
}

type ReqCheckObject struct {
	Checksum string
}

type ResCheckObject struct {
	Exists bool
	Err    error
}

type ReqCheckChunk struct {
	Checksum string
}

type ResCheckChunk struct {
	Exists bool
	Err    error
}

type ReqPurge struct {
	Uuid string
}

type ResPurge struct {
	Err error
}

type ReqClose struct {
	Uuid string
}

type ResClose struct {
	Err error
}

type ReqTransaction struct {
}

type ResTransaction struct {
	Uuid string
	Err  error
}

type ReqReferenceChunks struct {
	Transaction string
	Keys        []string
}

type ResReferenceChunks struct {
	Exists []bool
	Err    error
}

type ReqReferenceObjects struct {
	Transaction string
	Keys        []string
}

type ResReferenceObjects struct {
	Exists []bool
	Err    error
}

type ReqPutChunk struct {
	Transaction string
	Checksum    string
	Data        []byte
}

type ResPutChunk struct {
	Err error
}

type ReqPutObject struct {
	Transaction string
	Checksum    string
	Data        []byte
}

type ResPutObject struct {
	Err error
}

type ReqPutIndex struct {
	Transaction string
	Data        []byte
}

type ResPutIndex struct {
	Err error
}

type ReqCommit struct {
	Transaction string
}

type ResCommit struct {
	Err error
}

func ProtocolRegister() {
	gob.Register(storage.StoreConfig{})

	gob.Register(Request{})

	gob.Register(ReqChallenge{})
	gob.Register(ResChallenge{})

	gob.Register(ReqChallengeResponse{})
	gob.Register(ResChallengeResponse{})

	gob.Register(ReqCreate{})
	gob.Register(ResCreate{})

	gob.Register(ReqOpen{})
	gob.Register(ResOpen{})

	gob.Register(ReqGetIndexes{})
	gob.Register(ResGetIndexes{})

	gob.Register(ReqGetChunks{})
	gob.Register(ResGetChunks{})

	gob.Register(ReqGetObjects{})
	gob.Register(ResGetObjects{})

	gob.Register(ReqGetIndex{})
	gob.Register(ResGetIndex{})

	gob.Register(ReqGetObject{})
	gob.Register(ResGetObject{})

	gob.Register(ReqGetChunk{})
	gob.Register(ResGetChunk{})

	gob.Register(ReqCheckObject{})
	gob.Register(ResCheckObject{})

	gob.Register(ReqCheckChunk{})
	gob.Register(ResCheckChunk{})

	gob.Register(ReqPurge{})
	gob.Register(ResPurge{})

	gob.Register(ReqClose{})
	gob.Register(ResClose{})

	gob.Register(ReqTransaction{})
	gob.Register(ResTransaction{})

	gob.Register(ReqReferenceChunks{})
	gob.Register(ResReferenceChunks{})

	gob.Register(ReqReferenceObjects{})
	gob.Register(ResReferenceObjects{})

	gob.Register(ReqPutChunk{})
	gob.Register(ResPutChunk{})

	gob.Register(ReqPutObject{})
	gob.Register(ResPutObject{})

	gob.Register(ReqPutIndex{})
	gob.Register(ResPutIndex{})

	gob.Register(ReqCommit{})
	gob.Register(ResCommit{})
}
