package logsym

// Symfile is about creating and accessing a sym file.
// A symfile is a file of static information for a log.
// It does not vary over time but is constant to the source code file.
// It needs to be looked up frequently as the same bit of code is repeatedly executed.

import (
	"encoding/binary"
	"io"
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
	key       string
	valueType LogValueType
}

type SymbolEntry struct {
	symId       uint64
	numAccesses uint64
	message     string
	fname       string
	line        uint32
	keyTypeList []KeyType
}

type SymFile struct {
	entries   map[string]SymId // use message+fname+line as key
	file      *os.File
	nextSymId uint64
}

/*
 * Create a symbol file and allocate the SymFile data
 */
func SymFileCreate(baseFileName string) (sym *SymFile, err error) {
	f, err := os.Create(baseFileName + ".sym")
	if err != nil {
		return nil, err
	}
	entries := make(map[SymbolEntry]SymId)
	sym = &SymFile{
		file:    f,
		entries: entries,
	}
	return nil, sym
}

/*
 * Open sym file and read into map
 */
func SymFileOpen(baseFileName string) (sym *SymFile, err error) {

}

func (sym *SymFile) SymFileClose() error {
	return sym.file.Close()
}

/*
 * Add entry to map if doesn't exist already.
 * If new, then write a sym entry to the file
 */
func (sym *SymFile) SymFileAddEntry(entry SymbolEntry) (SymId, error) {
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

/*
 *	numAccesses uint64
 *	line    uint32
 *  messageLen uint32
 *	message string
 *  fnameLen uint32
 *	fname    string
 *  numKeys uint32
 *  type1   uint32
 *  key1Len uint32
 *  key1    variable
 *  type2   uint32
 *  key2len uint32
 *  key2    variable
 *  ...
 */
func (entry SymbolEntry) Write(w io.Writer) (length int, err error) {
	byteOrder := binary.LittleEndian
	err = binary.Write(w, byteOrder, entry.numAccesses)
	if err != nil {
		return 0, err
	}
	err = binary.Write(w, byteOrder, entry.line)
	if err != nil {
		return 0, err
	}
	msgBytes := []byte(entry.message)
	err = binary.Write(w, byteOrder, len(msgBytes))
	if err != nil {
		return 0, err
	}
	err = binary.Write(w, byteOrder, msgBytes)
	if err != nil {
		return 0, err
	}
	fnameBytes := []byte(entry.fname)
	err = binary.Write(w, byteOrder, len(fnameBytes))
	if err != nil {
		return 0, err
	}
	err = binary.Write(w, byteOrder, fnameBytes)
	if err != nil {
		return 0, err
	}
	numKeys := len(entry.keyTypeList)
	err = binary.Write(w, byteOrder, numKeys)
	if err != nil {
		return 0, err
	}
	for i := 0; i < numKeys; i++ {
		keyType := entry.keyTypeList[i]
		err = binary.Write(w, byteOrder, keyType.valueType)
		if err != nil {
			return 0, err
		}
		keyLen := len(keyType.key)
		err = binary.Write(w, byteOrder, keyLen)
		if err != nil {
			return 0, err
		}
		keyBytes := []byte(keyType.key)
		err = binary.Write(w, byteOrder, keyBytes)
		if err != nil {
			return 0, err
		}
	}
	// TODO: work out how many bytes written in total
}
