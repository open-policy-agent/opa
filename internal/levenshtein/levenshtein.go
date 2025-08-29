// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package levenshtein

import (
	"iter"
	"slices"

	"github.com/agnivade/levenshtein"
)

func ClosestStrings(minDistance int, a string, candidates iter.Seq[string]) []string {
	closestStrings := []string{}
	for c := range candidates {
		levDist := levenshtein.ComputeDistance(a, c)
		switch {
		case levDist < minDistance:
			closestStrings = []string{c}
			minDistance = levDist
		case levDist == minDistance:
			closestStrings = append(closestStrings, c)
			minDistance = levDist
		default:
			continue
		}
	}
	slices.Sort(closestStrings)
	return closestStrings
}
