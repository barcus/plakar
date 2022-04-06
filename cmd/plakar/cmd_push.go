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
	"os"

	"github.com/poolpOrg/plakar/logger"
	"github.com/poolpOrg/plakar/snapshot"
)

func init() {
	registerCommand("push", cmd_push)
}

func cmd_push(ctx Plakar, args []string) int {
	flags := flag.NewFlagSet("push", flag.ExitOnError)
	flags.Parse(args)

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}

	snap, err := snapshot.New(ctx.Store())
	if err != nil {
		logger.Error("%s", err)
		return 1
	}

	snap.Metadata.CommandLine = ctx.CommandLine

	if flags.NArg() == 0 {
		err = snap.Push([]string{dir})
	} else {
		err = snap.Push(flags.Args())
	}

	if err != nil {
		logger.Error("%s", err)
		return 1
	}

	logger.Info("created snapshot %s", snap.Metadata.Uuid)
	return 0
}
