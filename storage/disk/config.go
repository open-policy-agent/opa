// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"errors"
	"fmt"
	"os"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/open-policy-agent/opa/config"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

type cfg struct {
	Dir        string   `json:"directory"`
	AutoCreate bool     `json:"auto_create"`
	Partitions []string `json:"partitions"`
	Badger     string   `json:"badger"`
}

var ErrInvalidPartitionPath = errors.New("invalid storage path")

// OptionsFromConfig parses the passed config, extracts the disk storage
// settings, validates it, and returns a *Options struct pointer on success.
func OptionsFromConfig(raw []byte, id string) (*Options, error) {
	parsedConfig, err := config.ParseConfig(raw, id)
	if err != nil {
		return nil, err
	}

	if parsedConfig.Storage == nil || len(parsedConfig.Storage.Disk) == 0 {
		return nil, nil
	}

	var c cfg
	if err := util.Unmarshal(parsedConfig.Storage.Disk, &c); err != nil {
		return nil, err
	}

	if _, err := os.Stat(c.Dir); err != nil {
		if os.IsNotExist(err) && c.AutoCreate {
			err = os.MkdirAll(c.Dir, 0700) // overwrite err
		}
		if err != nil {
			return nil, fmt.Errorf("directory %v invalid: %w", c.Dir, err)
		}
	}

	opts := Options{
		Dir:    c.Dir,
		Badger: c.Badger,
	}
	for _, path := range c.Partitions {
		p, ok := storage.ParsePath(path)
		if !ok {
			return nil, fmt.Errorf("partition path '%v': %w", path, ErrInvalidPartitionPath)
		}
		opts.Partitions = append(opts.Partitions, p)
	}

	return &opts, nil
}

func badgerConfigFromOptions(opts Options) (badger.Options, error) {
	// Set some things _after_ FromSuperFlag to prohibit overriding them

	dir, err := dataDir(opts.Dir)
	if err != nil {
		return badger.DefaultOptions(""), err
	}

	return badger.DefaultOptions("").
			FromSuperFlag(opts.Badger).
			WithDir(dir).
			WithValueDir(dir).
			WithDetectConflicts(false), // We only allow one write txn at a time; so conflicts cannot happen.
		nil
}
