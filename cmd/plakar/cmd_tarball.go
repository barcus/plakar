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
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/poolpOrg/plakar/helpers"
	"github.com/poolpOrg/plakar/logger"
	"github.com/poolpOrg/plakar/storage"
)

func init() {
	registerCommand("tarball", cmd_tarball)
}

func cmd_tarball(ctx Plakar, store *storage.Store, args []string) int {
	var tarballPath string
	var tarballRebase bool

	flags := flag.NewFlagSet("tarball", flag.ExitOnError)
	flags.StringVar(&tarballPath, "output", fmt.Sprintf("plakar-%s.tar.gz", time.Now().UTC().Format(time.RFC3339)), "tarball pathname")
	flags.BoolVar(&tarballRebase, "rebase", false, "strip pathname when pulling")
	flags.Parse(args)

	if flags.NArg() == 0 {
		log.Fatalf("%s: need at least one snapshot ID to pull", flag.CommandLine.Name())
	}

	_, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	snapshots, err := getSnapshots(store, flags.Args())
	if err != nil {
		log.Fatal(err)
	}

	var gzipWriter *gzip.Writer
	if tarballPath == "-" {
		gzipWriter = gzip.NewWriter(os.Stdout)
	} else {
		fp, err := os.Create(tarballPath)
		if err != nil {
			log.Fatal(err)
		}
		gzipWriter = gzip.NewWriter(fp)
	}
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for offset, snapshot := range snapshots {
		_, prefix := parseSnapshotID(flags.Args()[offset])

		for file, checksum := range snapshot.Index.Pathnames {
			if prefix != "" {
				if !helpers.PathIsWithin(file, prefix) {
					continue
				}
			}

			info, _ := snapshot.LookupInodeForPathname(file)
			filepath := file
			if tarballRebase {
				filepath = strings.TrimPrefix(filepath, prefix)
			}
			header := &tar.Header{
				Name:    filepath,
				Size:    info.Size,
				Mode:    int64(info.Mode),
				ModTime: info.ModTime,
			}

			err = tarWriter.WriteHeader(header)
			if err != nil {
				logger.Error("could not write header for file %s", file)
				continue
			}

			obj := snapshot.LookupObjectForChecksum(checksum)
			for _, chunkChecksum := range obj.Chunks {
				data, err := snapshot.GetChunk(chunkChecksum)
				if err != nil {
					logger.Error("corrupted file %s", file)
					continue
				}

				_, err = io.WriteString(tarWriter, string(data))
				if err != nil {
					logger.Error("could not write file %s", file)
					continue
				}
			}

		}
	}

	if tarballPath != "-" {
		logger.Info("created tarball %s", tarballPath)
	}
	return 0
}
