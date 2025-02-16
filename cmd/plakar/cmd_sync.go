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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/poolpOrg/plakar/snapshot"
	"github.com/poolpOrg/plakar/storage"
)

func init() {
	registerCommand("sync", cmd_sync)
}

func cmd_sync(ctx Plakar, repository *storage.Repository, args []string) int {
	flags := flag.NewFlagSet("sync", flag.ExitOnError)
	flags.Parse(args)

	sourceRepository := repository
	sourceChunkChecksums, err := sourceRepository.GetChunks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not get chunks list from repository: %s\n", ctx.Repository, err)
		return 1
	}

	sourceObjectChecksums, err := sourceRepository.GetObjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not get objects list from repository: %s\n", ctx.Repository, err)
		return 1
	}

	sourceIndexes, err := sourceRepository.GetIndexes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not get indexes list from repository: %s\n", ctx.Repository, err)
		return 1
	}

	for _, repository := range flags.Args() {
		var syncRepository *storage.Repository
		if !strings.HasPrefix(repository, "/") {
			log.Fatalf("%s: does not support non filesystem plakar destinations for now", flag.CommandLine.Name())
			/*
				if strings.HasPrefix(repository, "plakar://") {
					syncrepository, _ = storage.New("client")
				} else if strings.HasPrefix(repository, "sqlite://") {
					syncrepository, _ = storage.New("database")
				} else {
					log.Fatalf("%s: unsupported plakar protocol", flag.CommandLine.Name())
				}
			*/
		}

		syncRepository, err = storage.Open(repository)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not open repository: %s\n", ctx.Repository, err)
			return 1
		}

		destChunkChecksums, err := syncRepository.GetChunks()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not get chunks list from repository: %s\n", ctx.Repository, err)
			return 1
		}

		destObjectChecksums, err := syncRepository.GetObjects()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not get objects list from repository: %s\n", ctx.Repository, err)
			return 1
		}

		destIndexes, err := syncRepository.GetIndexes()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not get indexes list from repository: %s\n", ctx.Repository, err)
			return 1
		}

		_ = sourceChunkChecksums
		_ = sourceObjectChecksums
		_ = sourceIndexes

		_ = destChunkChecksums
		_ = destObjectChecksums
		_ = destIndexes

		syncChunkChecksums := make([]string, 0)
		syncObjectChecksums := make([]string, 0)
		syncIndexes := make([]string, 0)

		for _, chunkChecksum := range sourceChunkChecksums {
			if !arrayContains(destChunkChecksums, chunkChecksum) {
				syncChunkChecksums = append(syncChunkChecksums, chunkChecksum)
			}
		}

		for _, objectChecksum := range sourceObjectChecksums {
			if !arrayContains(destObjectChecksums, objectChecksum) {
				syncObjectChecksums = append(syncObjectChecksums, objectChecksum)
			}
		}

		for _, index := range sourceIndexes {
			if !arrayContains(destIndexes, index) {
				syncIndexes = append(syncIndexes, index)
			}
		}

		for _, chunkChecksum := range syncChunkChecksums {
			data, err := sourceRepository.GetChunk(chunkChecksum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not get chunk from repository: %s\n", ctx.Repository, err)
				return 1
			}
			err = syncRepository.PutChunk(chunkChecksum, data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not write chunk to repository: %s\n", repository, err)
				return 1
			}
		}

		for _, objectChecksum := range syncObjectChecksums {
			data, err := sourceRepository.GetObject(objectChecksum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not get object from repository: %s\n", ctx.Repository, err)
				return 1
			}
			err = syncRepository.PutObject(objectChecksum, data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not write object to repository: %s\n", repository, err)
				return 1
			}
		}

		for _, index := range syncIndexes {
			data, err := sourceRepository.GetMetadata(index)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not get index from repository: %s\n", ctx.Repository, err)
				return 1
			}
			err = syncRepository.PutMetadata(index, data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not write object to repository: %s\n", repository, err)
				return 1
			}

			data, err = sourceRepository.GetIndex(index)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not get index from repository: %s\n", ctx.Repository, err)
				return 1
			}
			err = syncRepository.PutIndex(index, data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not write object to repository: %s\n", repository, err)
				return 1
			}

			snap, err := snapshot.Load(syncRepository, index)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: could not load index from repository: %s\n", repository, err)
				return 1
			}

			for _, chunk := range snap.Index.Chunks {
				err = syncRepository.ReferenceIndexChunk(index, chunk.Checksum)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: could not reference chunk in repository: %s\n", repository, err)
					return 1
				}
			}

			for _, object := range snap.Index.Objects {
				err = syncRepository.ReferenceIndexObject(index, object.Checksum)
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: could not reference object in repository: %s\n", repository, err)
					return 1
				}
			}

		}

	}

	return 0
}
