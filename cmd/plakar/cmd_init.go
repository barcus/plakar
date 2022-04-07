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

	"github.com/poolpOrg/plakar/cache"
	"github.com/poolpOrg/plakar/encryption"
	"github.com/poolpOrg/plakar/helpers"
	"github.com/poolpOrg/plakar/workdir"
)

func keypairGenerate() (string, []byte, error) {
	keypair, err := encryption.KeypairGenerate()
	if err != nil {
		return "", nil, err
	}

	var passphrase []byte
	for {
		passphrase, err = helpers.GetPassphraseConfirm("keypair")
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			continue
		}
		break
	}

	pem, err := keypair.Encrypt(passphrase)
	if err != nil {
		return "", nil, err
	}

	return keypair.Uuid, pem, err
}

func cmd_init(ctx Plakar, args []string) int {
	flags := flag.NewFlagSet("init", flag.ExitOnError)
	flags.Parse(args)

	if flags.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "%s: %s: too many parameters\n", flag.CommandLine.Name(), flags.Name())
		return 1
	}

	wd, err := workdir.Create(ctx.workdirPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %s: %s\n", flag.CommandLine.Name(), flags.Name(), ctx.workdirPath, err)
		return 1
	}

	err = cache.Create(ctx.cachePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: %s: %s\n", flag.CommandLine.Name(), flags.Name(), ctx.cachePath, err)
		return 1
	}

	_, keypair, err := keypairGenerate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: could not generate keypair: %s\n", flag.CommandLine.Name(), flags.Name(), err)
		return 1
	}
	err = wd.SaveEncryptedKeypair(keypair)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: could not save keypair: %s\n", flag.CommandLine.Name(), flags.Name(), err)
		return 1
	}

	return 0
}
