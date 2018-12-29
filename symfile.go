package logsym
// Symfile is about creating and accessing a sym file.
// A symfile is a file of static information for a log.
// It does not vary over time but is constant to the source code file.
// It needs to be looked up frequently as the same bit of code is repeatedly executed.

import (
	"os"
)

// Offset into the sym file for the symbol entry
type SymId uint64

// The various standard types supported
type LogValueType uint8 
const (
	TypeInt32 LogValueType = iota
	TypeUint32
	TypeInt64
	Typeuint64
	TypeDouble
    TypeByteData
	TypeStringNull
)

type KeyType struct {
	key string
	type LogValueType
}

type SymbolEntry struct {
	numAccesses uint64 
	message string
	file    string
	line    uint32
	keyTypeList []KeyType
}

type SymFile struct {
	entries map[SymbolEntry]SymId
	file *os.File
	nextSymId uint64
}

/*
 * Create a symbol file and allocate the SymFile data
 */
func SymFileCreate(baseFileName string) (sym *SymFile, error) {
	f, err := os.Create(baseFileName + ".sym")
    if err != nil {
		return err
	}
	entries := make(map[SymbolEntry]SymId)
    sym := &SymFile {
		file: f
		entries: entries
	} 
	return sym
}

/*
 * Open sym file and read into map
 */
func SymFileOpen(baseFileName string) (sym *SymFile, error) {

}

func (sym *SymFile)SymFileClose() (error) {
	return sym.file.Close() 
}

/*
 * Add entry to map if doesn't exist already.
 * If new, then write a sym entry to the file
 */
func (sym *SymFile)SymFileAddEntry(entry SymbolEntry) (SymId, error) {
	// If already there ...
	if symId, ok := sym.entries[entry]; ok {
		sym.numAccesses++
		return symId
	}
	// else new symbol entry
	len, err := entry.WriteFile(sym.file)
	if err != nil {
		return err
	}
	sym.nextSymId += len
	return sym.nextSymId
}