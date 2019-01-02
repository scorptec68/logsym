package logsym

// Symfile is about creating and accessing a sym file.
// A symfile is a file of static information for a log.
// It does not vary over time but is constant to the source code file.
// It needs to be looked up frequently as the same bit of code is repeatedly executed.

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
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

func symFileName(baseFileName string) string {
	return baseFileName + ".sym"
}

// SymFileCreate creates a symbol file and allocate the SymFile data
func SymFileCreate(baseFileName string) (sym *SymFile, err error) {
	f, err := os.Create(symFileName(baseFileName))
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

// SymFileReadin opens a sym file and read into map
func SymFileReadin(baseFileName string) (entries map[string]SymbolEntry, err error) {
	f, err := os.Open(symFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries = make(map[string]SymbolEntry)

	// read in all the entries starting at the start of the file
	reader := bufio.NewReader(f)
	for {
		var entry SymbolEntry
		err = entry.Read(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries[entry.keyString()] = entry
	}
	return entries, nil
}

// SymFileOpenAppend opens up the symFile, read in the entries and get ready for appending to file
func SymFileOpenAppend(baseFileName string) (sym *SymFile, err error) {
	entries, err := SymFileReadin(baseFileName)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(symFileName(baseFileName), os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	sym = &SymFile{file: f, entries: entries}
	return sym, nil
}

// SymFileClose closes the file
func (sym *SymFile) SymFileClose() error {
	return sym.file.Close()
}

// SymFileAddEntry adds an entry to map and to file if doesn't exist already.
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
	sym.entries[entry.keyString()] = entry

	return sym.nextSymID, nil
}

// Read the SymbolEntry using a reader
func (entry SymbolEntry) Read(r io.Reader) (err error) {
	byteOrder := binary.LittleEndian

	// read numAccesses
	err = binary.Read(r, byteOrder, &entry.numAccesses)
	if err != nil {
		return err
	}

	// read line number
	err = binary.Read(r, byteOrder, &entry.line)
	if err != nil {
		return err
	}

	// message string length
	var lenBytes uint32
	err = binary.Read(r, byteOrder, &lenBytes)
	if err != nil {
		return err
	}

	// message string
	msgBytes := make([]byte, lenBytes)
	n, err := r.Read(msgBytes)
	if err != nil {
		return err
	}
	if n != int(lenBytes) {
		return fmt.Errorf("Failed to read %v bytes for message", lenBytes)
	}
	entry.message = string(msgBytes[:lenBytes])

	// file name length
	err = binary.Read(r, byteOrder, &lenBytes)
	if err != nil {
		return err
	}

	// file name
	fnameBytes := make([]byte, lenBytes)
	n, err = r.Read(fnameBytes)
	if err != nil {
		return err
	}
	if n != int(lenBytes) {
		return fmt.Errorf("Failed to read %v bytes for file name", lenBytes)
	}
	entry.fname = string(fnameBytes[:lenBytes])

	// read in all the key type pairs
	var numKeys uint32
	err = binary.Read(r, byteOrder, &numKeys)
	if err != nil {
		return err
	}

	entry.keyTypeList = make([]KeyType, numKeys)
	for i := 0; i < int(numKeys); i++ {
		var keyType KeyType

		// read value type
		err = binary.Read(r, byteOrder, &keyType.valueType)
		if err != nil {
			return err
		}

		// read key length
		var keyLen uint32
		err = binary.Read(r, byteOrder, &keyLen)
		if err != nil {
			return err
		}

		// read key string data
		keyBytes := make([]byte, keyLen)
		n, err = r.Read(keyBytes)
		if err != nil {
			return err
		}
		if n != int(lenBytes) {
			return fmt.Errorf("Failed to read %v bytes for key data", lenBytes)
		}
		keyType.key = string(keyBytes)

		entry.keyTypeList[i] = keyType
	}

	return nil
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
 *  type1   uint8
 *  key1Len uint32
 *  key1    []byte
 *  type2   uint8
 *  key2len uint32
 *  key2    []byte
 *  ...
 */
// Write() writes the symbol entry using the writer
func (entry SymbolEntry) Write(w io.Writer) (length int, err error) {
	byteOrder := binary.LittleEndian

	length += binary.Size(entry.numAccesses)
	err = binary.Write(w, byteOrder, entry.numAccesses)
	if err != nil {
		return 0, err
	}

	length += binary.Size(entry.line)
	err = binary.Write(w, byteOrder, entry.line)
	if err != nil {
		return 0, err
	}

	msgBytes := []byte(entry.message)
	var lenBytes uint32
	lenBytes = uint32(len(msgBytes))

	// write length
	length += binary.Size(lenBytes)
	err = binary.Write(w, byteOrder, lenBytes)
	if err != nil {
		return 0, err
	}

	// write actual bytes
	length += int(lenBytes)
	err = binary.Write(w, byteOrder, msgBytes)
	if err != nil {
		return 0, err
	}

	fnameBytes := []byte(entry.fname)
	lenBytes = uint32(len(fnameBytes))

	// write length
	length += binary.Size(lenBytes)
	err = binary.Write(w, byteOrder, lenBytes)
	if err != nil {
		return 0, err
	}

	// write actual bytes
	length += int(lenBytes)
	err = binary.Write(w, byteOrder, fnameBytes)
	if err != nil {
		return 0, err
	}

	// write num keys
	var numKeys uint32
	numKeys = uint32(len(entry.keyTypeList))
	length += binary.Size(numKeys)
	err = binary.Write(w, byteOrder, numKeys)
	if err != nil {
		return 0, err
	}

	for i := 0; i < int(numKeys); i++ {

		// write type
		keyType := entry.keyTypeList[i]
		length += binary.Size(keyType.valueType)
		err = binary.Write(w, byteOrder, keyType.valueType)
		if err != nil {
			return 0, err
		}

		// write key length
		var keyLen uint32
		keyLen = uint32(len(keyType.key))
		length += binary.Size(keyLen)
		err = binary.Write(w, byteOrder, keyLen)
		if err != nil {
			return 0, err
		}

		// write key bytes
		keyBytes := []byte(keyType.key)
		length += int(keyLen)
		err = binary.Write(w, byteOrder, keyBytes)
		if err != nil {
			return 0, err
		}
	}
	return length, nil
}
