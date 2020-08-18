package wasmer

import (
	"unsafe"
)

// WasiVersion represents the WASI version.
type WasiVersion uint

const (
	// Unknown represents an unknown WASI version.
	Unknown = WasiVersion(cVersionUnknown)

	// Latest represents the latest WASI version.
	Latest = WasiVersion(cVersionSnapshot0)

	// Snapshot0 represents the `wasi_unstable` WASI version.
	Snapshot0 = WasiVersion(cVersionSnapshot0)

	// Snapshot1 represents the `wasi_snapshot1_preview` WASI version.
	Snapshot1 = WasiVersion(cVersionSnapshot1)
)

// MapDirEntry is an entry that can be passed to `NewWasiImportObject`.
// Preopens a file for the WASI module but renames it to the given name
type MapDirEntry struct {
	alias    string
	hostPath string
}

// NewDefaultWasiImportObject constructs a new `ImportObject`
// with WASI host imports.
//
// To specify WASI program arguments, environment variables,
// preopened directories, and more, see `NewWasiImportObject`
func NewDefaultWasiImportObject() *ImportObject {
	return NewDefaultWasiImportObjectForVersion(Latest)
}

// NewDefaultWasiImportObjectForVersion is similar to
// `NewDefaultWasiImportObject` but it specifies the WASI version.
func NewDefaultWasiImportObjectForVersion(version WasiVersion) *ImportObject {
	var inner = cNewWasmerWasiImportObjectForVersion((uint)(version), nil, 0, nil, 0, nil, 0, nil, 0)

	return &ImportObject{inner}
}

// NewWasiImportObject creates an `ImportObject` with the default WASI imports.
// Specify arguments (the first is the program name),
// environment variables ("envvar=value"), preopened directories
// (host file paths), and mapped directories (host file paths with an
// alias, see `MapDirEntry`)
func NewWasiImportObject(
	arguments []string,
	environmentVariables []string,
	preopenedDirs []string,
	mappedDirs []MapDirEntry,
) *ImportObject {
	return NewWasiImportObjectForVersion(
		Latest,
		arguments,
		environmentVariables,
		preopenedDirs,
		mappedDirs,
	)
}

// NewWasiImportObjectForVersion is similar to `NewWasiImportObject`
// but it specifies the WASI version.
func NewWasiImportObjectForVersion(
	version WasiVersion,
	arguments []string,
	environmentVariables []string,
	preopenedDirs []string,
	mappedDirs []MapDirEntry,
) *ImportObject {
	var argumentsBytes = []cWasmerByteArray{}

	for _, argument := range arguments {
		argumentsBytes = append(argumentsBytes, cGoStringToWasmerByteArray(argument))
	}

	var environmentVariablesBytes = []cWasmerByteArray{}

	for _, env := range environmentVariables {
		environmentVariablesBytes = append(environmentVariablesBytes, cGoStringToWasmerByteArray(env))
	}

	var preopenedDirsBytes = []cWasmerByteArray{}

	for _, preopenedDir := range preopenedDirs {
		preopenedDirsBytes = append(preopenedDirsBytes, cGoStringToWasmerByteArray(preopenedDir))
	}
	var mappedDirsBytes = []cWasmerWasiMapDirEntryT{}

	for _, mappedDir := range mappedDirs {
		var wasiMappedDir = cAliasAndHostPathToWasiDirEntry(mappedDir.alias, mappedDir.hostPath)
		mappedDirsBytes = append(mappedDirsBytes, wasiMappedDir)
	}

	var inner = cNewWasmerWasiImportObject(
		(*cWasmerByteArray)(unsafe.Pointer(&argumentsBytes)),
		(uint)(len(argumentsBytes)),
		(*cWasmerByteArray)(unsafe.Pointer(&environmentVariablesBytes)),
		(uint)(len(environmentVariablesBytes)),
		(*cWasmerByteArray)(unsafe.Pointer(&preopenedDirsBytes)),
		(uint)(len(preopenedDirsBytes)),
		(*cWasmerWasiMapDirEntryT)(unsafe.Pointer(&mappedDirsBytes)),
		(uint)(len(mappedDirsBytes)),
	)

	return &ImportObject{inner}
}

// WasiGetVersion returns the WASI version of a module if any, other
// `Unknown` is returned.
func WasiGetVersion(module Module) WasiVersion {
	return (WasiVersion)(cWasmerWasiGetVersion(
		module.module,
	))
}
