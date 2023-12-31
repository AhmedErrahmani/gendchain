package rlp

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sync"
)

var (
	// Common encoded values.
	// These are useful when implementing EncodeRLP.
	EmptyString = []byte{0x80}
	EmptyList   = []byte{0xC0}
)

const (
	// MaxHeadSize is the maximum size for a header, in bytes.
	MaxHeadSize = 9

	// MaxBigIntSize is the maximum size for a BigInt, in bytes.
	// This is used so we don't have to fully compute big.Int.Bytes().
	//
	// We determine this value by using total currency in circulation plus
	// additional padding. This can hold 10^77.
	MaxBigIntSize = 32
)

// Encoder is implemented by types that require custom
// encoding rules or want to encode private fields.
type Encoder interface {
	// EncodeRLP should write the RLP encoding of its receiver to w.
	// If the implementation is a pointer method, it may also be
	// called for nil pointers.
	//
	// Implementations should generate valid RLP. The data written is
	// not verified at the moment, but a future version might. It is
	// recommended to write only a single value but writing multiple
	// values or no value at all is also permitted.
	EncodeRLP(io.Writer) error
}

// SliceEncoder is implemented by types that require custom encoding rules for
// their elements.
type SliceEncoder interface {
	EncodeRLPElem(int, io.Writer) error
}

// Encode writes the RLP encoding of val to w. Note that Encode may
// perform many small writes in some cases. Consider making w
// buffered.
//
// Encode uses the following type-dependent encoding rules:
//
// If the type implements the Encoder interface, Encode calls
// EncodeRLP. This is true even for nil pointers, please see the
// documentation for Encoder.
//
// To encode a pointer, the value being pointed to is encoded. For nil
// pointers, Encode will encode the zero value of the type. A nil
// pointer to a struct type always encodes as an empty RLP list.
// A nil pointer to an array encodes as an empty list (or empty string
// if the array has element type byte).
//
// Struct values are encoded as an RLP list of all their encoded
// public fields. Recursive struct types are supported.
//
// To encode slices and arrays, the elements are encoded as an RLP
// list of the value's elements. Note that arrays and slices with
// element type uint8 or byte are always encoded as an RLP string.
//
// A Go string is encoded as an RLP string.
//
// An unsigned integer value is encoded as an RLP string. Zero always
// encodes as an empty RLP string. Encode also supports *big.Int.
//
// An interface value encodes as the value contained in the interface.
//
// Boolean values are not supported, nor are signed integers, floating
// point numbers, maps, channels and functions.
func Encode(w io.Writer, val interface{}) error {
	if outer, ok := w.(*encbuf); ok {
		// Encode was called by some type's EncodeRLP.
		// Avoid copying by writing to the outer encbuf directly.
		return outer.encode(val)
	}
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return err
	}
	return eb.toWriter(w)
}

// EncodeBytes returns the RLP encoding of val.
// Please see the documentation of Encode for the encoding rules.
func EncodeToBytes(val interface{}) ([]byte, error) {
	eb := encbufPool.Get().(*encbuf)
	defer encbufPool.Put(eb)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return nil, err
	}
	return eb.toBytes(), nil
}

// EncodeReader returns a reader from which the RLP encoding of val
// can be read. The returned size is the total size of the encoded
// data.
//
// Please see the documentation of Encode for the encoding rules.
func EncodeToReader(val interface{}) (size int, r io.Reader, err error) {
	eb := encbufPool.Get().(*encbuf)
	eb.reset()
	if err := eb.encode(val); err != nil {
		return 0, nil, err
	}
	return eb.size(), &encReader{buf: eb}, nil
}

type encbuf struct {
	str     []byte      // string data, contains everything except list headers
	lheads  []*listhead // all list headers
	lhsize  int         // sum of sizes of all encoded list headers
	sizebuf []byte      // 9-byte auxiliary buffer for uint encoding
}

type listhead struct {
	offset int // index of this header in string data
	size   int // total size of encoded data (including list headers)
}

// encode writes head to the given buffer, which must be at least
// 9 bytes long. It returns the encoded bytes.
func (head *listhead) encode(buf []byte) []byte {
	return buf[:puthead(buf, 0xC0, 0xF7, uint64(head.size))]
}

// headsize returns the size of a list or string header
// for a value of the given size.
func headsize(size uint64) int {
	if size < 56 {
		return 1
	}
	return 1 + intsize(size)
}

// puthead writes a list or string header to buf.
// buf must be at least 9 bytes long.
func puthead(buf []byte, smalltag, largetag byte, size uint64) int {
	if size < 56 {
		buf[0] = smalltag + byte(size)
		return 1
	} else {
		sizesize := putint(buf[1:], size)
		buf[0] = largetag + byte(sizesize)
		return sizesize + 1
	}
}

// encbufs are pooled.
var encbufPool = sync.Pool{
	New: func() interface{} { return &encbuf{sizebuf: make([]byte, 9)} },
}

func (w *encbuf) reset() {
	w.lhsize = 0
	if w.str != nil {
		w.str = w.str[:0]
	}
	if w.lheads != nil {
		w.lheads = w.lheads[:0]
	}
}

// encbuf implements io.Writer so it can be passed it into EncodeRLP.
func (w *encbuf) Write(b []byte) (int, error) {
	w.str = append(w.str, b...)
	return len(b), nil
}

func (w *encbuf) encode(val interface{}) error {
	rval := reflect.ValueOf(val)
	t := rval.Type()
	ti, err := cachedTypeInfo(t, tags{})
	if err != nil {
		return err
	}
	return ti.writer(rval, w)
}

func (w *encbuf) encodeStringHeader(size int) {
	if size < 56 {
		w.str = append(w.str, 0x80+byte(size))
	} else {
		// TODO: encode to w.str directly
		sizesize := putint(w.sizebuf[1:], uint64(size))
		w.sizebuf[0] = 0xB7 + byte(sizesize)
		w.str = append(w.str, w.sizebuf[:sizesize+1]...)
	}
}

func (w *encbuf) encodeString(b []byte) {
	if len(b) == 1 && b[0] <= 0x7F {
		// fits single byte, no string header
		w.str = append(w.str, b[0])
	} else {
		w.encodeStringHeader(len(b))
		w.str = append(w.str, b...)
	}
}

func (w *encbuf) list() *listhead {
	lh := &listhead{offset: len(w.str), size: w.lhsize}
	w.lheads = append(w.lheads, lh)
	return lh
}

func (w *encbuf) listEnd(lh *listhead) {
	lh.size = w.size() - lh.offset - lh.size
	if lh.size < 56 {
		w.lhsize += 1 // length encoded into kind tag
	} else {
		w.lhsize += 1 + intsize(uint64(lh.size))
	}
}

func (w *encbuf) size() int {
	return len(w.str) + w.lhsize
}

func (w *encbuf) toBytes() []byte {
	out := make([]byte, w.size())
	strpos := 0
	pos := 0
	for _, head := range w.lheads {
		// write string data before header
		n := copy(out[pos:], w.str[strpos:head.offset])
		pos += n
		strpos += n
		// write the header
		enc := head.encode(out[pos:])
		pos += len(enc)
	}
	// copy string data after the last list header
	copy(out[pos:], w.str[strpos:])
	return out
}

func (w *encbuf) toWriter(out io.Writer) (err error) {
	strpos := 0
	for _, head := range w.lheads {
		// write string data before header
		if head.offset-strpos > 0 {
			n, err := out.Write(w.str[strpos:head.offset])
			strpos += n
			if err != nil {
				return err
			}
		}
		// write the header
		enc := head.encode(w.sizebuf)
		if _, err = out.Write(enc); err != nil {
			return err
		}
	}
	if strpos < len(w.str) {
		// write string data after the last list header
		_, err = out.Write(w.str[strpos:])
	}
	return err
}

// encReader is the io.Reader returned by EncodeToReader.
// It releases its encbuf at EOF.
type encReader struct {
	buf    *encbuf // the buffer we're reading from. this is nil when we're at EOF.
	lhpos  int     // index of list header that we're reading
	strpos int     // current position in string buffer
	piece  []byte  // next piece to be read
}

func (r *encReader) Read(b []byte) (n int, err error) {
	for {
		if r.piece = r.next(); r.piece == nil {
			// Put the encode buffer back into the pool at EOF when it
			// is first encountered. Subsequent calls still return EOF
			// as the error but the buffer is no longer valid.
			if r.buf != nil {
				encbufPool.Put(r.buf)
				r.buf = nil
			}
			return n, io.EOF
		}
		nn := copy(b[n:], r.piece)
		n += nn
		if nn < len(r.piece) {
			// piece didn't fit, see you next time.
			r.piece = r.piece[nn:]
			return n, nil
		}
		r.piece = nil
	}
}

// next returns the next piece of data to be read.
// it returns nil at EOF.
func (r *encReader) next() []byte {
	switch {
	case r.buf == nil:
		return nil

	case r.piece != nil:
		// There is still data available for reading.
		return r.piece

	case r.lhpos < len(r.buf.lheads):
		// We're before the last list header.
		head := r.buf.lheads[r.lhpos]
		sizebefore := head.offset - r.strpos
		if sizebefore > 0 {
			// String data before header.
			p := r.buf.str[r.strpos:head.offset]
			r.strpos += sizebefore
			return p
		} else {
			r.lhpos++
			return head.encode(r.buf.sizebuf)
		}

	case r.strpos < len(r.buf.str):
		// String data at the end, after all list headers.
		p := r.buf.str[r.strpos:]
		r.strpos = len(r.buf.str)
		return p

	default:
		return nil
	}
}

var (
	encoderInterface      = reflect.TypeOf(new(Encoder)).Elem()
	sliceEncoderInterface = reflect.TypeOf(new(SliceEncoder)).Elem()
	big0                  = big.NewInt(0)
)

// makeWriter creates a writer function for the given type.
func makeWriter(typ reflect.Type, ts tags) (writer, error) {
	kind := typ.Kind()
	switch {
	case typ == rawValueType:
		return writeRawValue, nil
	case typ.Implements(encoderInterface):
		return writeEncoder, nil
	case typ.Implements(sliceEncoderInterface):
		return writeSliceEncoder(ts), nil
	case kind != reflect.Ptr && reflect.PtrTo(typ).Implements(encoderInterface):
		return writeEncoderNoPtr, nil
	case kind == reflect.Interface:
		return writeInterface, nil
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return writeBigIntPtr, nil
	case typ.AssignableTo(bigInt):
		return writeBigIntNoPtr, nil
	case isUint(kind):
		return writeUint, nil
	case kind == reflect.Bool:
		return writeBool, nil
	case kind == reflect.String:
		return writeString, nil
	case kind == reflect.Slice && isByte(typ.Elem()):
		return writeBytes, nil
	case kind == reflect.Array && isByte(typ.Elem()):
		return writeByteArray, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return makeSliceWriter(typ, ts)
	case kind == reflect.Struct:
		return makeStructWriter(typ)
	case kind == reflect.Ptr:
		return makePtrWriter(typ)
	default:
		return nil, fmt.Errorf("rlp: type %v is not RLP-serializable", typ)
	}
}

func isByte(typ reflect.Type) bool {
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}

func writeRawValue(val reflect.Value, w *encbuf) error {
	w.str = append(w.str, val.Bytes()...)
	return nil
}

func WriteRawValueTo(w io.Writer, v RawValue) (n int64, err error) {
	nn, err := w.Write(v)
	return int64(nn), err
}

func RawValueSize(v RawValue) int {
	return len(v)
}

func writeUint(val reflect.Value, w *encbuf) error {
	i := val.Uint()
	if i == 0 {
		w.str = append(w.str, 0x80)
	} else if i < 128 {
		// fits single byte
		w.str = append(w.str, byte(i))
	} else {
		// TODO: encode int to w.str directly
		s := putint(w.sizebuf[1:], i)
		w.sizebuf[0] = 0x80 + byte(s)
		w.str = append(w.str, w.sizebuf[:s+1]...)
	}
	return nil
}

func WriteUint64To(w io.Writer, v uint64) (n int64, err error) {
	if v == 0 {
		nn, err := w.Write([]byte{0x80})
		return int64(nn), err
	} else if v < 128 {
		// fits single byte
		nn, err := w.Write([]byte{byte(v)})
		return int64(nn), err
	}

	// Encode value to bytes.
	buf := make([]byte, 9)
	sz := putint(buf[1:], v)
	buf[0] = 0x80 + byte(sz)

	nn, err := w.Write(buf[:sz+1])
	return int64(nn), err
}

func AppendUint64(b []byte, v uint64) []byte {
	if v == 0 {
		buf := b[len(b):][:1]
		buf[0] = 0x80
		return b[:len(b)+1]
	} else if v < 128 {
		buf := b[len(b):][:1]
		buf[0] = byte(v)
		return b[:len(b)+1]
	}
	return AppendHead(b, 0x80, v)
}

func Uint64Size(v uint64) int {
	if v < 128 {
		return 1
	}
	return intsize(v) + 1
}

func writeBool(val reflect.Value, w *encbuf) error {
	if val.Bool() {
		w.str = append(w.str, 0x01)
	} else {
		w.str = append(w.str, 0x80)
	}
	return nil
}

func WriteBoolTo(w io.Writer, v bool) (n int64, err error) {
	if v {
		nn, err := w.Write([]byte{0x01})
		return int64(nn), err
	}
	nn, err := w.Write([]byte{0x80})
	return int64(nn), err
}

func AppendBool(b []byte, v bool) []byte {
	buf := b[len(b):][:1]
	if v {
		buf[0] = 0x01
	} else {
		buf[0] = 0x80
	}
	return b[:len(b)+1]
}

func BoolSize(v uint64) int {
	return 1
}

func writeBigIntPtr(val reflect.Value, w *encbuf) error {
	ptr := val.Interface().(*big.Int)
	if ptr == nil {
		w.str = append(w.str, 0x80)
		return nil
	}
	return writeBigInt(ptr, w)
}

func writeBigIntNoPtr(val reflect.Value, w *encbuf) error {
	i := val.Interface().(big.Int)
	return writeBigInt(&i, w)
}

func writeBigInt(i *big.Int, w *encbuf) error {
	if cmp := i.Cmp(big0); cmp == -1 {
		return fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if cmp == 0 {
		w.str = append(w.str, 0x80)
	} else {
		w.encodeString(i.Bytes())
	}
	return nil
}

func WriteBigIntTo(w io.Writer, v *big.Int) (n int64, err error) {
	if v == nil {
		nn, err := w.Write([]byte{0x80})
		return int64(nn), err
	} else if cmp := v.Cmp(big0); cmp == 0 {
		nn, err := w.Write([]byte{0x80})
		return int64(nn), err
	} else if cmp == -1 {
		return 0, fmt.Errorf("rlp: cannot write negative *big.Int")
	}
	return WriteBytesTo(w, v.Bytes())
}

func AppendBigInt(b []byte, v *big.Int) ([]byte, error) {
	if v == nil {
		buf := b[len(b):][:1]
		buf[0] = 0x80
		return b[:len(b)+1], nil
	} else if cmp := v.Cmp(big0); cmp == 0 {
		buf := b[len(b):][:1]
		buf[0] = 0x80
		return b[:len(b)+1], nil
	} else if cmp == -1 {
		return nil, fmt.Errorf("rlp: cannot append negative *big.Int")
	}
	return AppendBytes(b, v.Bytes()), nil
}

func BigIntSize(v *big.Int) int {
	if v == nil {
		return 1
	} else if cmp := v.Cmp(big0); cmp == 0 {
		return 1
	} else if cmp == -1 {
		return 0
	}
	return BytesSize(v.Bytes())
}

func writeBytes(val reflect.Value, w *encbuf) error {
	w.encodeString(val.Bytes())
	return nil
}

func writeByteArray(val reflect.Value, w *encbuf) error {
	if !val.CanAddr() {
		// Slice requires the value to be addressable.
		// Make it addressable by copying.
		copy := reflect.New(val.Type()).Elem()
		copy.Set(val)
		val = copy
	}
	size := val.Len()
	slice := val.Slice(0, size).Bytes()
	w.encodeString(slice)
	return nil
}

func writeString(val reflect.Value, w *encbuf) error {
	s := val.String()
	if len(s) == 1 && s[0] <= 0x7f {
		// fits single byte, no string header
		w.str = append(w.str, s[0])
	} else {
		w.encodeStringHeader(len(s))
		w.str = append(w.str, s...)
	}
	return nil
}

func WriteBytesTo(w io.Writer, v []byte) (n int64, err error) {
	// Fits single byte, no string header
	if len(v) == 1 && v[0] <= 0x7F {
		nn, err := w.Write(v)
		return int64(nn), err
	}

	// Write header.
	if len(v) < 56 {
		nn, err := w.Write([]byte{0x80 + byte(len(v))})
		if n += int64(nn); err != nil {
			return n, err
		}
	} else {
		buf := make([]byte, 9)
		sz := putint(buf[1:], uint64(len(v)))
		buf[0] = 0xB7 + byte(sz)
		nn, err := w.Write(buf[:sz+1])
		if n += int64(nn); err != nil {
			return n, err
		}
	}

	nn, err := w.Write(v)
	n += int64(nn)
	return n, err
}

func AppendBytes(b, v []byte) []byte {
	// Fits single byte, no string header
	if len(v) == 1 && v[0] <= 0x7F {
		buf := b[len(b):][:1]
		buf[0] = v[0]
		return b[:len(b)+1]
	}

	// Write small header, if possible.
	if len(v) < 56 {
		buf := b[len(b):][:1+len(v)]
		buf[0] = 0x80 + byte(len(v))
		copy(buf[1:], v)
		return b[:len(b)+1+len(v)]
	}

	b = AppendHead(b, 0xB7, uint64(len(v)))
	b = b[:len(b)+len(v)]
	copy(b[len(b)-len(v):], v)
	return b
}

func BytesSize(v []byte) int {
	if len(v) == 1 && v[0] <= 0x7F {
		return 1
	} else if len(v) < 56 {
		return 1 + len(v)
	}
	return intsize(uint64(len(v))) + 1 + len(v)
}

func WriteListHeaderTo(w io.Writer, v int) (n int64, err error) {
	if v < 56 {
		nn, err := w.Write([]byte{0xC0 + byte(v)})
		return int64(nn), err
	}

	buf := make([]byte, 9)
	sz := putint(buf[1:], uint64(v))
	buf[0] = 0xF7 + byte(sz)
	nn, err := w.Write(buf[:sz+1])
	return int64(nn), err
}

// PrependListHeader writes the list header to the beginning of b.
// This function assumes that MaxHeadSize has been allocated at the beginning already.
// Returns the byte slice starting from the beginning of the header.
func PrependListHeader(b []byte) []byte {
	_ = b[8] // bounds check

	sz := len(b) - MaxHeadSize
	if sz < 56 {
		b[8] = 0xC0 + byte(sz)
		return b[8:]
	}

	b = b[MaxHeadSize-(intsize(uint64(sz))+1):]
	AppendHead(b[:0], 0xF7, uint64(sz))
	return b
}

func ListHeaderSize(sz int) int {
	return headsize(uint64(sz))
}

func writeEncoder(val reflect.Value, w *encbuf) error {
	return val.Interface().(Encoder).EncodeRLP(w)
}

// writeEncoderNoPtr handles non-pointer values that implement Encoder
// with a pointer receiver.
func writeEncoderNoPtr(val reflect.Value, w *encbuf) error {
	if !val.CanAddr() {
		// We can't get the address. It would be possible to make the
		// value addressable by creating a shallow copy, but this
		// creates other problems so we're not doing it (yet).
		//
		// package json simply doesn't call MarshalJSON for cases like
		// this, but encodes the value as if it didn't implement the
		// interface. We don't want to handle it that way.
		return fmt.Errorf("rlp: game over: unadressable value of type %v, EncodeRLP is pointer method", val.Type())
	}
	return val.Addr().Interface().(Encoder).EncodeRLP(w)
}

func writeInterface(val reflect.Value, w *encbuf) error {
	if val.IsNil() {
		// Write empty list. This is consistent with the previous RLP
		// encoder that we had and should therefore avoid any
		// problems.
		w.str = append(w.str, 0xC0)
		return nil
	}
	eval := val.Elem()
	ti, err := cachedTypeInfo(eval.Type(), tags{})
	if err != nil {
		return err
	}
	return ti.writer(eval, w)
}

func writeSliceEncoder(ts tags) writer {
	return func(val reflect.Value, w *encbuf) error {
		if !ts.tail {
			defer w.listEnd(w.list())
		}
		s := val.Interface().(SliceEncoder)
		vlen := val.Len()
		for i := 0; i < vlen; i++ {
			if err := s.EncodeRLPElem(i, w); err != nil {
				return err
			}
		}
		return nil
	}
}

func makeSliceWriter(typ reflect.Type, ts tags) (writer, error) {
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}
	writer := func(val reflect.Value, w *encbuf) error {
		if !ts.tail {
			defer w.listEnd(w.list())
		}
		vlen := val.Len()
		for i := 0; i < vlen; i++ {
			if err := etypeinfo.writer(val.Index(i), w); err != nil {
				return err
			}
		}
		return nil
	}
	return writer, nil
}

func makeStructWriter(typ reflect.Type) (writer, error) {
	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}
	writer := func(val reflect.Value, w *encbuf) error {
		lh := w.list()
		for _, f := range fields {
			if err := f.info.writer(val.Field(f.index), w); err != nil {
				return err
			}
		}
		w.listEnd(lh)
		return nil
	}
	return writer, nil
}

func makePtrWriter(typ reflect.Type) (writer, error) {
	etypeinfo, err := cachedTypeInfo1(typ.Elem(), tags{})
	if err != nil {
		return nil, err
	}

	// determine nil pointer handler
	var nilfunc func(*encbuf) error
	kind := typ.Elem().Kind()
	switch {
	case kind == reflect.Array && isByte(typ.Elem().Elem()):
		nilfunc = func(w *encbuf) error {
			w.str = append(w.str, 0x80)
			return nil
		}
	case kind == reflect.Struct || kind == reflect.Array:
		nilfunc = func(w *encbuf) error {
			// encoding the zero value of a struct/array could trigger
			// infinite recursion, avoid that.
			w.listEnd(w.list())
			return nil
		}
	default:
		zero := reflect.Zero(typ.Elem())
		nilfunc = func(w *encbuf) error {
			return etypeinfo.writer(zero, w)
		}
	}

	writer := func(val reflect.Value, w *encbuf) error {
		if val.IsNil() {
			return nilfunc(w)
		} else {
			return etypeinfo.writer(val.Elem(), w)
		}
	}
	return writer, err
}

// putint writes i to the beginning of b in big endian byte
// order, using the least number of bytes needed to represent i.
func putint(b []byte, i uint64) (size int) {
	switch {
	case i < (1 << 8):
		b[0] = byte(i)
		return 1
	case i < (1 << 16):
		b[0] = byte(i >> 8)
		b[1] = byte(i)
		return 2
	case i < (1 << 24):
		b[0] = byte(i >> 16)
		b[1] = byte(i >> 8)
		b[2] = byte(i)
		return 3
	case i < (1 << 32):
		b[0] = byte(i >> 24)
		b[1] = byte(i >> 16)
		b[2] = byte(i >> 8)
		b[3] = byte(i)
		return 4
	case i < (1 << 40):
		b[0] = byte(i >> 32)
		b[1] = byte(i >> 24)
		b[2] = byte(i >> 16)
		b[3] = byte(i >> 8)
		b[4] = byte(i)
		return 5
	case i < (1 << 48):
		b[0] = byte(i >> 40)
		b[1] = byte(i >> 32)
		b[2] = byte(i >> 24)
		b[3] = byte(i >> 16)
		b[4] = byte(i >> 8)
		b[5] = byte(i)
		return 6
	case i < (1 << 56):
		b[0] = byte(i >> 48)
		b[1] = byte(i >> 40)
		b[2] = byte(i >> 32)
		b[3] = byte(i >> 24)
		b[4] = byte(i >> 16)
		b[5] = byte(i >> 8)
		b[6] = byte(i)
		return 7
	default:
		b[0] = byte(i >> 56)
		b[1] = byte(i >> 48)
		b[2] = byte(i >> 40)
		b[3] = byte(i >> 32)
		b[4] = byte(i >> 24)
		b[5] = byte(i >> 16)
		b[6] = byte(i >> 8)
		b[7] = byte(i)
		return 8
	}
}

func AppendHead(b []byte, prefix byte, v uint64) []byte {
	switch {
	case v < (1 << 8):
		buf := b[len(b):][:2]
		buf[0] = prefix + 1
		buf[1] = byte(v)
		return b[:len(b)+2]
	case v < (1 << 16):
		buf := b[len(b):][:3]
		buf[0] = prefix + 2
		buf[1] = byte(v >> 8)
		buf[2] = byte(v)
		return b[:len(b)+3]
	case v < (1 << 24):
		buf := b[len(b):][:4]
		buf[0] = prefix + 3
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
		return b[:len(b)+4]
	case v < (1 << 32):
		buf := b[len(b):][:5]
		buf[0] = prefix + 4
		buf[1] = byte(v >> 24)
		buf[2] = byte(v >> 16)
		buf[3] = byte(v >> 8)
		buf[4] = byte(v)
		return b[:len(b)+5]
	case v < (1 << 40):
		buf := b[len(b):][:6]
		buf[0] = prefix + 5
		buf[1] = byte(v >> 32)
		buf[2] = byte(v >> 24)
		buf[3] = byte(v >> 16)
		buf[4] = byte(v >> 8)
		buf[5] = byte(v)
		return b[:len(b)+6]
	case v < (1 << 48):
		buf := b[len(b):][:7]
		buf[0] = prefix + 6
		buf[1] = byte(v >> 40)
		buf[2] = byte(v >> 32)
		buf[3] = byte(v >> 24)
		buf[4] = byte(v >> 16)
		buf[5] = byte(v >> 8)
		buf[6] = byte(v)
		return b[:len(b)+7]
	case v < (1 << 56):
		buf := b[len(b):][:8]
		buf[0] = prefix + 7
		buf[1] = byte(v >> 48)
		buf[2] = byte(v >> 40)
		buf[3] = byte(v >> 32)
		buf[4] = byte(v >> 24)
		buf[5] = byte(v >> 16)
		buf[6] = byte(v >> 8)
		buf[7] = byte(v)
		return b[:len(b)+8]
	default:
		buf := b[len(b):][:9]
		buf[0] = prefix + 8
		buf[1] = byte(v >> 56)
		buf[2] = byte(v >> 48)
		buf[3] = byte(v >> 40)
		buf[4] = byte(v >> 32)
		buf[5] = byte(v >> 24)
		buf[6] = byte(v >> 16)
		buf[7] = byte(v >> 8)
		buf[8] = byte(v)
		return b[:len(b)+9]
	}
}

// intsize computes the minimum number of bytes required to store i.
func intsize(i uint64) (size int) {
	for size = 1; ; size++ {
		if i >>= 8; i == 0 {
			return size
		}
	}
}
