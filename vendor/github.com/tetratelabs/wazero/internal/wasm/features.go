package wasm

import (
	"fmt"
	"strings"
)

// Features are the currently enabled features.
//
// Note: This is a bit flag until we have too many (>63). Flags are simpler to manage in multiple places than a map.
type Features uint64

// Features20191205 include those finished in WebAssembly 1.0 (20191205).
//
// See https://www.w3.org/TR/2019/REC-wasm-core-1-20191205
const Features20191205 = FeatureMutableGlobal

// Features20220419 include those finished in WebAssembly 2.0 (20220419).
//
// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#release-1-1
const Features20220419 = Features20191205 |
	FeatureBulkMemoryOperations |
	FeatureMultiValue |
	FeatureNonTrappingFloatToIntConversion |
	FeatureReferenceTypes |
	FeatureSignExtensionOps |
	FeatureSIMD

const (
	// FeatureBulkMemoryOperations decides if parsing should succeed on the following instructions:
	//
	// * [ OpcodeMiscPrefix, OpcodeMiscMemoryInit]
	// * [ OpcodeMiscPrefix, OpcodeMiscDataDrop]
	// * [ OpcodeMiscPrefix, OpcodeMiscMemoryCopy]
	// * [ OpcodeMiscPrefix, OpcodeMiscMemoryFill]
	// * [ OpcodeMiscPrefix, OpcodeMiscTableInit]
	// * [ OpcodeMiscPrefix, OpcodeMiscElemDrop]
	// * [ OpcodeMiscPrefix, OpcodeMiscTableCopy]
	//
	// Also, if the parsing should succeed with the presence of SectionIDDataCount.
	//
	// See https://www.w3.org/TR/2022/WD-wasm-core-2-20220419/appendix/changes.html#bulk-memory-and-table-instructions
	FeatureBulkMemoryOperations Features = 1 << iota

	// FeatureMultiValue decides if parsing should succeed on the following:
	//
	// * FunctionType.Results length greater than one.
	// * `block`, `loop` and `if` can be arbitrary function types.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/multi-value/Overview.md
	FeatureMultiValue

	// FeatureMutableGlobal decides if global vars are allowed to be imported or exported (ExternTypeGlobal)
	// See https://github.com/WebAssembly/mutable-global
	FeatureMutableGlobal

	// FeatureNonTrappingFloatToIntConversion decides if parsing should succeed on the following instructions:
	//
	// * [ OpcodeMiscPrefix, OpcodeMiscI32TruncSatF32S]
	// * [ OpcodeMiscPrefix, OpcodeMiscI32TruncSatF32U]
	// * [ OpcodeMiscPrefix, OpcodeMiscI64TruncSatF32S]
	// * [ OpcodeMiscPrefix, OpcodeMiscI64TruncSatF32U]
	// * [ OpcodeMiscPrefix, OpcodeMiscI32TruncSatF64S]
	// * [ OpcodeMiscPrefix, OpcodeMiscI32TruncSatF64U]
	// * [ OpcodeMiscPrefix, OpcodeMiscI64TruncSatF64S]
	// * [ OpcodeMiscPrefix, OpcodeMiscI64TruncSatF64U]
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/nontrapping-float-to-int-conversion/Overview.md
	FeatureNonTrappingFloatToIntConversion

	// FeatureReferenceTypes enables various features related to reference types and tables.
	// * Introduction of new values types: RefTypeFuncref and RefTypeExternref
	// * Support for the following opcodes:
	//  * OpcodeRefNull
	//  * OpcodeRefIsNull
	//  * OpcodeRefFunc
	// 	* OpcodeTableGet
	// 	* OpcodeTableSet
	// 	* [ OpcodeMiscPrefix, OpcodeMiscTableFill]
	// 	* [ OpcodeMiscPrefix, OpcodeMiscTableGrow]
	// 	* [ OpcodeMiscPrefix, OpcodeMiscTableSize]
	// * Support for multiple tables per module:
	// 	* OpcodeCallIndirect, OpcodeTableInit, and OpcodeElemDrop can take non-zero table indexes.
	// 	* Element segments can take non-zero table index.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/reference-types/Overview.md
	FeatureReferenceTypes

	// FeatureSignExtensionOps decides if parsing should succeed on the following instructions:
	//
	// * OpcodeI32Extend8S
	// * OpcodeI32Extend16S
	// * OpcodeI64Extend8S
	// * OpcodeI64Extend16S
	// * OpcodeI64Extend32S
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/sign-extension-ops/Overview.md
	FeatureSignExtensionOps

	// FeatureSIMD enables the vector value type and vector instructions.
	//
	// See https://github.com/WebAssembly/spec/blob/main/proposals/simd/SIMD.md
	FeatureSIMD
)

// Set assigns the value for the given feature.
func (f Features) Set(feature Features, val bool) Features {
	if val {
		return f | feature
	}
	return f &^ feature
}

// Get returns the value of the given feature.
func (f Features) Get(feature Features) bool {
	return f&feature != 0
}

// Require fails with a configuration error if the given feature is not enabled
func (f Features) Require(feature Features) error {
	if f&feature == 0 {
		return fmt.Errorf("feature %q is disabled", feature)
	}
	return nil
}

// String implements fmt.Stringer by returning each enabled feature.
func (f Features) String() string {
	var builder strings.Builder
	for i := 0; i <= 63; i++ { // cycle through all bits to reduce code and maintenance
		target := Features(1 << i)
		if f.Get(target) {
			if name := featureName(target); name != "" {
				if builder.Len() > 0 {
					builder.WriteByte('|')
				}
				builder.WriteString(name)
			}
		}
	}
	return builder.String()
}

func featureName(f Features) string {
	switch f {
	case FeatureMutableGlobal:
		// match https://github.com/WebAssembly/mutable-global
		return "mutable-global"
	case FeatureSignExtensionOps:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/sign-extension-ops/Overview.md
		return "sign-extension-ops"
	case FeatureMultiValue:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/multi-value/Overview.md
		return "multi-value"
	case FeatureNonTrappingFloatToIntConversion:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/nontrapping-float-to-int-conversion/Overview.md
		return "nontrapping-float-to-int-conversion"
	case FeatureBulkMemoryOperations:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/bulk-memory-operations/Overview.md
		return "bulk-memory-operations"
	case FeatureReferenceTypes:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/reference-types/Overview.md
		return "reference-types"
	case FeatureSIMD:
		// match https://github.com/WebAssembly/spec/blob/main/proposals/simd/SIMD.md
		return "simd"
	}
	return ""
}
