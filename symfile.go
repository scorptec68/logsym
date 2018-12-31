package logsym

// Symfile is about creating and accessing a sym file.
// A symfile is a file of static information for a log.
// It does not vary over time but is constant to the source code file.
// It needs to be looked up frequently as the same bit of code is repeatedly executed.

import (
	"encoding/binary"
	"io"
	"os"
	"strconv"
	"unsafe"
)

// SymID is an offset into the sym file for the symbol entry
type SymID uint64

// The LogValueType consists of various standard types
type LogValueType uint8

// Value types supported in log
const (
	TypeInt32 LogValueType = iota
	TypeUint32
	TypeInt64
	Typeuint64
	TypeDouble
	TypeByteData
	TypeStringNull
)

// KeyType consists of a key and its associated type
type KeyType struct {
	key       string
	valueType LogValueType
}

// SymbolEntry is the in memory version of a .sym file entry
type SymbolEntry struct {
	symID       SymID
	numAccesses uint64
	message     string
	fname       string
	line        uint32
	keyTypeList []KeyType
}

// SymFile is the top level object for the .sym file
type SymFile struct {
	entries   map[string]SymbolEntry // use message+fname+line as key
	file      *os.File
	nextSymID SymID
}

// string of symbol entry used for key
func (entry SymbolEntry) keyString() string {
	return entry.message + entry.fname + strconv.Itoa(int(entry.line))
}

// SymFileCreate creates a symbol file and allocate the SymFile data
func SymFileCreate(baseFileName string) (sym *SymFile, err error) {
	f, err := os.Create(baseFileName + ".sym")
	if err != nil {
		return nil, err
	}
	entries := make(map[string]SymbolEntry)
	sym = &SymFile{
		file:    f,
		entries: entries,
	}
	return sym, nil
}

// SymFileOpen opens a sym file and read into map
func SymFileOpen(baseFileName string) (sym *SymFile, err error) {

}

// SymFileClose closes the file
func (sym *SymFile) SymFileClose() error {
	return sym.file.Close()
}

// SymFileAddEntry adds an entry to map if doesn't exist already.
// If new, then write a sym entry to the file
func (sym *SymFile) SymFileAddEntry(entry SymbolEntry) (SymID, error) {
	// If already there ...
	if entry, ok := sym.entries[entry.keyString()]; ok {
		entry.numAccesses++
		return entry.symID, nil
	}
	// else new symbol entry
	len, err := entry.Write(sym.file)
	if err != nil {
		return 0, err
	}
	sym.nextSymID += SymID(len)
	return sym.nextSymID, nil
}

/*
 * Format:
 *	numAccesses uint64
 *	line    uint32
 *
 *  messageLen uint32
 *	message []byte
 *
 *  fnameLen uint32
 *	fname    []byte
 *
 *  numKeys uint32
 *  type1   uint32
 *  key1Len uint32
 *  key1    []byte
 *  type2   uint32
 *  key2len uint32
 *  key2    []byte
 *  ...
 */
// Write() writes the symbol entry using the writer
func (entry SymbolEntry) Write(w io.Writer) (length uint32, err error) {
	byteOrder := binary.LittleEndian

	length += uint32(unsafe.Sizeof(entry.numAccesses))
	err = binary.Write(w, byteOrder, entry.numAccesses)
	if err != nil {
		return 0, err
	}

	length += uint32(unsafe.Sizeof(entry.line))
	err = binary.Write(w, byteOrder, entry.line)
	if err != nil {
		return 0, err
	}

	msgBytes := []byte(entry.message)
	var lenBytes uint32
	lenBytes = uint32(len(msgBytes))

	// write length
	length += uint32(unsafe.Sizeof(lenBytes))
	err = binary.Write(w, byteOrder, lenBytes)
	if err != nil {
		return 0, err
	}

	// write actual bytes
	length += lenBytes
	err = binary.Write(w, byteOrder, msgBytes)
	if err != nil {
		return 0, err
	}

	fnameBytes := []byte(entry.fname)
	lenBytes = uint32(len(fnameBytes))

	// write length
	length += uint32(unsafe.Sizeof(lenBytes))
	err = binary.Write(w, byteOrder, lenBytes)
	if err != nil {
		return 0, err
	}

	// write actual bytes
	length += lenBytes
	err = binary.Write(w, byteOrder, fnameBytes)
	if err != nil {
		return 0, err
	}

	// write num keys
	var numKeys uint32
	numKeys = uint32(len(entry.keyTypeList))
	length += uint32(unsafe.Sizeof(numKeys))
	err = binary.Write(w, byteOrder, numKeys)
	if err != nil {
		return 0, err
	}

	for i := 0; i < int(numKeys); i++ {

		// write type
		keyType := entry.keyTypeList[i]
		length += uint32(unsafe.Sizeof(keyType.valueType))
		err = binary.Write(w, byteOrder, keyType.valueType)
		if err != nil {
			return 0, err
		}

		// write key length
		var keyLen uint32
		keyLen = uint32(len(keyType.key))
		length += uint32(unsafe.Sizeof(keyLen))
		err = binary.Write(w, byteOrder, keyLen)
		if err != nil {
			return 0, err
		}

		// write key bytes
		keyBytes := []byte(keyType.key)
		length += keyLen
		err = binary.Write(w, byteOrder, keyBytes)
		if err != nil {
			return 0, err
		}
	}
	return length, nil
}
