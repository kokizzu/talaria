// Copyright 2019 Grabtaxi Holdings PTE LTE (GRAB), All rights reserved.
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file

package block

import (
	"sort"

	"github.com/grab/talaria/internal/encoding/orc"
	"github.com/grab/talaria/internal/presto"
	"github.com/kelindar/binary/nocopy"
)

// FromOrc ...
func FromOrc(key string, b []byte) (block *Block, err error) {
	i, err := orc.FromBuffer(b)
	if err != nil {
		return nil, err
	}

	// Get the list of columns in the ORC file
	var columns []string
	schema := i.Schema()
	for k := range schema {
		columns = append(columns, k)
	}

	// Sort the columns for consistency
	sort.Strings(columns)

	// Create presto columns
	blocks := make(map[string]presto.Column, len(columns))
	index := make([]string, 0, len(columns))
	for _, c := range columns {
		if typ, hasType := schema[c]; hasType {
			blocks[c] = presto.NewColumn(typ)
			index = append(index, c)
		}
	}

	// Create a block
	block = new(Block)
	block.Key = nocopy.String(key)
	i.Range(func(i int, row []interface{}) bool {
		for i, v := range row {
			blocks[index[i]].Append(v)
		}
		return false
	}, columns...)

	// Write the columns into the block
	if err := block.writeColumns(blocks); err != nil {
		return nil, err
	}
	return
}

// FromOrcBy decodes a set of blocks from an orc file and repartitions
// it by the specified partition key.
func FromOrcBy(payload []byte, partitionBy string) ([]Block, error) {
	const chunks = 25000

	result := make([]Block, 0, 16)
	_, err := orc.SplitByColumn(payload, partitionBy, func(key string, columnChunk []byte) bool {
		_, splitErr := orc.SplitBySize(columnChunk, chunks, func(chunk []byte) bool {
			blk, err := FromOrc(key, chunk)
			if err != nil {
				return true
			}

			result = append(result, *blk)
			return false
		})
		return splitErr != nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}