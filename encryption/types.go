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

package encryption

import (
	"crypto/ed25519"
	"time"
)

type Keypair struct {
	CreationTime time.Time
	Uuid         string
	PrivateKey   ed25519.PrivateKey
	PublicKey    ed25519.PublicKey
	Key          []byte
}

type SerializedKeypair struct {
	CreationTime time.Time
	Uuid         string
	PrivateKey   string
	PublicKey    string
	Key          string
}

type PublicKey struct {
	CreationTime time.Time
	Uuid         string
	PublicKey    ed25519.PublicKey
}

type SerializedPublicKey struct {
	CreationTime time.Time
	Uuid         string
	PublicKey    string
}

type Secret struct {
	CreationTime time.Time
	Uuid         string
	Key          []byte
}

type SerializedSecret struct {
	CreationTime time.Time
	Uuid         string
	Key          string
}
