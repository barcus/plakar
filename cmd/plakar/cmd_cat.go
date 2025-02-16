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
	"io"
	"os"

	"github.com/poolpOrg/plakar/logger"
	"github.com/poolpOrg/plakar/storage"
)

func init() {
	registerCommand("cat", cmd_cat)
}

func cmd_cat(ctx Plakar, repository *storage.Repository, args []string) int {
	flags := flag.NewFlagSet("cat", flag.ExitOnError)
	flags.Parse(args)

	if flags.NArg() == 0 {
		logger.Error("%s: at least one parameter is required", flags.Name())
		return 1
	}

	snapshots, err := getSnapshots(repository, flags.Args())
	if err != nil {
		logger.Error("%s: could not obtain snapshots list: %s", flags.Name(), err)
		return 1
	}

	errors := 0
	for offset, snapshot := range snapshots {
		_, pathname := parseSnapshotID(flags.Args()[offset])

		if pathname == "" {
			logger.Error("%s: missing filename for snapshot %s", flags.Name(), snapshot.Metadata.Uuid)
			errors++
			continue
		}

		rd, err := snapshot.NewReader(pathname)
		if err != nil {
			logger.Error("%s: %s: %s", flags.Name(), pathname, err)
			errors++
			continue
		}

		_, err = io.Copy(os.Stdout, rd)
		if err != nil {
			logger.Error("%s: %s: %s", flags.Name(), pathname, err)
			errors++
			continue
		}
	}

	if errors != 0 {
		return 1
	}
	return 0
}
