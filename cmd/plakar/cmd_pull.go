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
	"log"
	"os"

	"github.com/poolpOrg/plakar/storage"
)

func init() {
	registerCommand("pull", cmd_pull)
}

func cmd_pull(ctx Plakar, store *storage.Store, args []string) int {
	var pullPath string
	var pullRebase bool

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	flags := flag.NewFlagSet("pull", flag.ExitOnError)
	flags.StringVar(&pullPath, "path", dir, "base directory where pull will restore")
	flags.BoolVar(&pullRebase, "rebase", false, "strip pathname when pulling")
	flags.Parse(args)

	if flags.NArg() == 0 {
		log.Fatalf("%s: need at least one snapshot ID to pull", flag.CommandLine.Name())
	}

	snapshots, err := getSnapshots(store, flags.Args())
	if err != nil {
		log.Fatal(err)
	}

	for offset, snapshot := range snapshots {
		_, pattern := parseSnapshotID(flags.Args()[offset])
		snapshot.Pull(pullPath, pullRebase, pattern)
	}

	return 0
}
