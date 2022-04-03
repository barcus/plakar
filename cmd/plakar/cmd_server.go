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

	"github.com/poolpOrg/plakar/network"
)

func init() {
	registerCommand("server", cmd_server)
}

func cmd_server(ctx Plakar, args []string) int {
	var baseDirectory string

	flags := flag.NewFlagSet("server", flag.ExitOnError)
	flags.StringVar(&baseDirectory, "basedir", "", "base directory")
	flags.Parse(args)

	if baseDirectory == "" {
		log.Fatal("need base directory")
	}

	addr := ":9876"
	if flags.NArg() == 1 {
		addr = flags.Arg(0)
	}

	network.Server(ctx.Store(), addr, baseDirectory)
	return 0
}
