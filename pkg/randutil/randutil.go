// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package randutil implements random utilities.
package randutil

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

var lll = int64(len(ll))

func String(n int) string {
	b := make([]byte, n)
	for i := range b {
		nBig, err := rand.Int(rand.Reader, big.NewInt(lll))
		if err != nil {
			panic(err)
		}
		b[i] = ll[nBig.Int64()]
	}

	nBig, err := rand.Int(rand.Reader, big.NewInt(randomsN))
	if err != nil {
		panic(err)
	}

	pfx := randoms[nBig.Int64()]
	s := pfx + string(b)
	if len(s) > n {
		s = s[:n]
	}

	return s
}

func Bytes(n int) []byte {
	return []byte(String(n))
}

// openssl rand -hex 32.
func Hex(n int) string {
	return hex.EncodeToString(Bytes(n))
}

var randomsN = int64(len(randoms))

var randoms = []string{
	"autumn",
	"sun",
	"splendid",
	"sunny",
	"original",
	"dream",
	"whole",
	"aws",
	"amazon",
	"flow",
	"cherry",
	"grand",
	"tree",
	"frost",
	"deluxe",
	"superb",
	"morning",
	"grand",
	"sparkling",
	"wandering",
	"summertime",
	"butterfly",
	"boldly",
	"green",
	"river",
	"breeze",
	"hiking",
	"proud",
	"great",
	"mochi",
	"floral",
	"spectacular",
	"dune",
	"modern",
	"delight",
	"lively",
	"forte",
	"waterfall",
	"embark",
	"flower",
	"roadtrip",
	"atlas",
	"grass",
	"haze",
	"spotlight",
	"glacial",
	"mountain",
	"snowflake",
	"misty",
	"summer",
	"good",
	"icy",
	"coffee",
	"awesome",
	"spring",
	"twilight",
	"blue",
	"coral",
	"everest",
	"galaxy",
	"hello",
	"seattle",
	"wind",
	"breeze",
	"watermelon",
	"sea",
	"ocean",
	"kirkland",
	"bellevue",
	"sunrise",
	"waterfront",
	"magnificent",
	"exclusive",
	"tropical",
	"morning",
	"sunset",
	"blueshift",
	"dynamic",
	"forest",
	"impressive",
	"amelia",
	"amzn",
	"rufus",
	"spheres",
	"innovation",
	"apple",
	"inventive",
	"brazil",
	"milan",
	"cloud",
	"rustc",
	"sun",
	"sound",
	"sky",
	"surf",
	"island",
	"water",
	"wildflower",
	"wave",
	"charisma",
	"water",
	"amber",
	"reinvent",
	"oscar",
	"integrity",
	"accountable",
	"day1",
	"prime",
	"nitro",
	"maria",
	"frosty",
	"paper",
	"star",
	"onion",
	"linux",
	"rust",
	"hawaii",
	"otter",
	"varzea",
	"obidos",
}
