package logsym

// Symfile is about creating and accessing a sym file.
// A symfile is a file of static information for a log.
// It does not vary over time but is constant based on the source code file.
// The Sym data needs to be looked up frequently as the same bit of source code is repeatedly executed.

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// SymID is an offset into the sym file for the symbol entry
type SymID uint64

// The LogValueType consists of various standard types
type LogValueType uint8

// LogLevel is the level of logging eg. debug or info
type LogLevel uint8

// Value types supported in log
const (
	TypeUint8 LogValueType = iota
	TypeInt8
	TypeInt32
	TypeUint32
	TypeInt64
	TypeUint64
	TypeFloat32
	TypeFloat64
	TypeBoolean
	TypeString
	TypeByteData
)

// Levels for debugging
const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
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
	level       LogLevel
	message     string
	fname       string
	line        uint32
	keyTypeList []KeyType
}

/*
 * SymbolEntry on-disk format:
 *
 *	symID       uint64
 *	numAccesses uint64
 *  level       uint8
 *	line        uint32
 *
 *  messageLen uint32
 *	message    []byte
 *
 *  fnameLen uint32
 *	fname    []byte
 *
 *  numKeys uint32
 *
 *  type1   uint8
 *  key1Len uint32
 *  key1    []byte
 *
 *  type2   uint8
 *  key2len uint32
 *  key2    []byte
 *  ...
 */

// SymFile is the top level object for the .sym file
type SymFile struct {
	msg2Entries map[string]*SymbolEntry // use message+fname+line as key - seen entry before?
	id2Entries  map[SymID]*SymbolEntry  // symbold id as key - get info about symbol such as typeList
	file        *os.File                // file used for appending symbols to the end
	nextSymID   SymID                   // the next id/offset for the next symbol to be added
}

// string of symbol entry used for key
func (entry SymbolEntry) keyString() string {
	return entry.message + entry.fname + strconv.Itoa(int(entry.line))
}

// create a string version of the symbol file for debugging etc...
func (sym SymFile) String() string {
	var str strings.Builder
	fmt.Fprintf(&str, "SymFile\n")
	fmt.Fprintf(&str, "  nextSymId: %v\n", sym.nextSymID)
	fmt.Fprintf(&str, "  entries:\n")

	// sort keys
	var keys []string
	for k := range sym.msg2Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// output values in key sorted order
	for _, k := range keys {
		fmt.Fprintf(&str, "    Key: \"%v\"\n", k)
		fmt.Fprintf(&str, "    Value: %v\n", sym.msg2Entries[k])
	}
	return str.String()
}

func (entry SymbolEntry) String() string {
	return fmt.Sprintf("Entry<symId: %v, numAccesses: %v, level: %v, message: \"%v\", fname: \"%v\", line: %v, keyTypes: %v>",
		entry.symID, entry.numAccesses, entry.level, entry.message, entry.fname, entry.line, entry.keyTypeList)
}

func (keyType KeyType) String() string {
	return fmt.Sprintf("<key: \"%v\", type: %v>", keyType.key, keyType.valueType)
}

func (valueType LogValueType) String() string {
	switch valueType {
	case TypeBoolean:
		return "Boolean"
	case TypeInt8:
		return "Int8"
	case TypeInt32:
		return "Int32"
	case TypeInt64:
		return "Int64"
	case TypeUint8:
		return "Uint8"
	case TypeUint32:
		return "Uint32"
	case TypeUint64:
		return "Uint64"
	case TypeFloat32:
		return "Float32"
	case TypeFloat64:
		return "Float64"
	case TypeString:
		return "String"
	case TypeByteData:
		return "ByteData"
	default:
		return "Unknown type"
	}
}

func (level LogLevel) String() string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelError:
		return "ERROR"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	default:
		return "Unknown Log Level"
	}
}

// The file name of the symbol file
func symFileName(baseFileName string) string {
	return baseFileName + ".sym"
}

// SymFileRemove deletes the sym file given base name
func SymFileRemove(baseFileName string) error {
	return os.Remove(symFileName(baseFileName))
}

// SymFileCreate creates a new symbol file and allocates the SymFile data
func SymFileCreate(baseFileName string) (sym *SymFile, err error) {
	f, err := os.Create(symFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	msg2Entries := make(map[string]*SymbolEntry)
	id2Entries := make(map[SymID]*SymbolEntry)
	sym = &SymFile{
		file:        f,
		msg2Entries: msg2Entries,
		id2Entries: id2Entries,
	}
	return sym, nil
}

// symFileReadin reads a whole sym file into a map
func symFileReadin(baseFileName string)
			(msg2Entries map[string]*SymbolEntry, 
			 id2Entries map[SymID]*SymbolEntry,
			 err error) {
	f, err := os.Open(symFileName(baseFileName))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	msg2Entries = make(map[string]*SymbolEntry)
	id2Entries = make(map[SymID]*SymbolEntry)

	// read in all the entries from the start of the file
	reader := bufio.NewReader(f)
	for {
		var entry SymbolEntry
		err = entry.Read(reader)
		if err == io.EOF {
			return entries, nil
		}
		if err != nil {
			return nil, err
		}
		//fmt.Printf("entry: %v\n", entry)
		msg2Entries[entry.keyString()] = &entry
		id2Entries[entry.symID] = &entry
	}
}

// SymFileOpenAppend opens up the symFile, read in the entries and get ready for appending to file
func SymFileOpenAppend(baseFileName string) (sym *SymFile, err error) {
	msg2entries, id2Entries, err := symFileReadin(baseFileName)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(symFileName(baseFileName), os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	sym = &SymFile{file: f, msg2Entries: msg2Entries, id2Entries: id2Entries, nextSymID: SymID(stat.Size())}
	return sym, nil
}

// SymFileClose closes the file
func (sym *SymFile) SymFileClose() error {
	return sym.file.Close()
}

// CreateSymbolEntry creates a symbol entry
func CreateSymbolEntry(message string, filename string, line uint32, keyTypes []KeyType) SymbolEntry {
	return SymbolEntry{message: message, fname: filename, line: line, keyTypeList: keyTypes}
}

// SymFileAddEntry adds an entry to map and to file if doesn't exist already.
func (sym *SymFile) SymFileAddEntry(entry SymbolEntry) (SymID, error) {
	// If already there ...
	if entry, ok := sym.msg2Entries[entry.keyString()]; ok {
		entry.numAccesses++
		return entry.symID, nil
	}

	// else new symbol entry
	entry.symID = sym.nextSymID
	len, err := entry.Write(sym.file)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("len of write entry: %v\n", len)

	sym.nextSymID += SymID(len)
	sym.msg2Entries[entry.keyString()] = &entry
	sym.id2Entries[entry.symID] = &entry

	//fmt.Printf("sym.nextSymID: %v\n", sym.nextSymID)
	return sym.nextSymID, nil
}

func (sym *SymFile) SymFileGetEntry(id SymID) (entry SymbolEntry, ok bool) {
	return sym.id2Entries[id]
}

// Read the SymbolEntry using a reader
func (entry *SymbolEntry) Read(r io.Reader) (err error) {
	byteOrder := binary.LittleEndian

	// read numAccesses
	err = binary.Read(r, byteOrder, &entry.symID)
	if err != nil {
		return err
	}
	//fmt.Printf("symID: %v\n", entry.symID)

	// read numAccesses
	err = binary.Read(r, byteOrder, &entry.numAccesses)
	if err != nil {
		return err
	}
	//fmt.Printf("numAccesses: %v\n", entry.numAccesses)

	// read level
	err = binary.Read(r, byteOrder, &entry.level)
	if err != nil {
		return err
	}
	//fmt.Printf("level: %v\n", entry.level)

	// read line number
	err = binary.Read(r, byteOrder, &entry.line)
	if err != nil {
		return err
	}
	//fmt.Printf("line: %v\n", entry.line)

	// message string length
	var lenBytes uint32
	err = binary.Read(r, byteOrder, &lenBytes)
	if err != nil {
		return err
	}
	//fmt.Printf("msg len: %v\n", lenBytes)

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
	//fmt.Printf("msg: %v\n", entry.message)

	// file name length
	err = binary.Read(r, byteOrder, &lenBytes)
	if err != nil {
		return err
	}
	//fmt.Printf("fname len: %v\n", lenBytes)

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
	//fmt.Printf("fname: %v\n", entry.fname)

	// read in all the key type pairs
	var numKeys uint32
	err = binary.Read(r, byteOrder, &numKeys)
	if err != nil {
		return err
	}
	//fmt.Printf("num keys: %v\n", numKeys)

	entry.keyTypeList = make([]KeyType, numKeys)
	for i := 0; i < int(numKeys); i++ {
		var keyType KeyType

		// read value type
		err = binary.Read(r, byteOrder, &keyType.valueType)
		if err != nil {
			return err
		}
		//fmt.Printf("type: %v\n", keyType.valueType)

		// read key length
		var keyLen uint32
		err = binary.Read(r, byteOrder, &keyLen)
		if err != nil {
			return err
		}
		//fmt.Printf("key len: %v\n", keyLen)

		// read key string data
		keyBytes := make([]byte, keyLen)
		n, err = r.Read(keyBytes)
		if err != nil {
			return err
		}
		if n != int(keyLen) {
			return fmt.Errorf("Failed to read %v bytes for key data got %v bytes", keyLen, n)
		}
		keyType.key = string(keyBytes)
		//fmt.Printf("key: %v\n", keyType.key)

		entry.keyTypeList[i] = keyType
	}

	return nil
}

// Write() writes the symbol entry using the writer
func (entry SymbolEntry) Write(w io.Writer) (length int, err error) {
	byteOrder := binary.LittleEndian

	length += binary.Size(entry.symID)
	err = binary.Write(w, byteOrder, entry.symID)
	if err != nil {
		return 0, err
	}
	//fmt.Printf("symId: %v\n", entry.symID)

	length += binary.Size(entry.numAccesses)
	err = binary.Write(w, byteOrder, entry.numAccesses)
	if err != nil {
		return 0, err
	}

	length += binary.Size(entry.level)
	err = binary.Write(w, byteOrder, entry.level)
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
