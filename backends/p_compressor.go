package backends

import (
	"bytes"
	"compress/zlib"
	"github.com/cskwrd/go-guerrilla/mail"
	"io"
	"sync"
)

// ----------------------------------------------------------------------------------
// Processor Name: compressor
// ----------------------------------------------------------------------------------
// Description   : Compress the e.Data (email data) and e.DeliveryHeader together
// ----------------------------------------------------------------------------------
// Config Options: None
// --------------:-------------------------------------------------------------------
// Input         : e.Data, e.DeliveryHeader generated by Header() processor
// ----------------------------------------------------------------------------------
// Output        : sets the pointer to a compressor in e.Info["zlib-compressor"]
//               : to write the compressed data, simply use fmt to print as a string,
//               : eg. fmt.Println("%s", e.Info["zlib-compressor"])
//               : or just call the String() func .Info["zlib-compressor"].String()
//               : Note that it can only be outputted once. It destroys the buffer
//               : after being printed
// ----------------------------------------------------------------------------------
func init() {
	processors["compressor"] = func() Decorator {
		return Compressor()
	}
}

// compressedData struct will be compressed using zlib when printed via fmt
type DataCompressor struct {
	ExtraHeaders []byte
	Data         *bytes.Buffer
	// the pool is used to recycle buffers to ease up on the garbage collector
	Pool *sync.Pool
}

// newCompressedData returns a new CompressedData
func newCompressor() *DataCompressor {
	// grab it from the pool
	var p = sync.Pool{
		// if not available, then create a new one
		New: func() interface{} {
			var b bytes.Buffer
			return &b
		},
	}
	return &DataCompressor{
		Pool: &p,
	}
}

// Set the extraheaders and buffer of data to compress
func (c *DataCompressor) set(b []byte, d *bytes.Buffer) {
	c.ExtraHeaders = b
	c.Data = d
}

// String implements the Stringer interface.
// Can only be called once!
// This is because the compression buffer will be reset and compressor will be returned to the pool
func (c *DataCompressor) String() string {
	if c.Data == nil {
		return ""
	}
	//borrow a buffer form the pool
	b := c.Pool.Get().(*bytes.Buffer)
	// put back in the pool
	defer func() {
		b.Reset()
		c.Pool.Put(b)
	}()

	var r *bytes.Reader
	w, _ := zlib.NewWriterLevel(b, zlib.BestSpeed)
	r = bytes.NewReader(c.ExtraHeaders)
	_, _ = io.Copy(w, r)
	_, _ = io.Copy(w, c.Data)
	_ = w.Close()
	return b.String()
}

// clear it, without clearing the pool
func (c *DataCompressor) clear() {
	c.ExtraHeaders = []byte{}
	c.Data = nil
}

func Compressor() Decorator {
	return func(p Processor) Processor {
		return ProcessWith(func(e *mail.Envelope, task SelectTask) (Result, error) {
			if task == TaskSaveMail {
				compressor := newCompressor()
				compressor.set([]byte(e.DeliveryHeader), &e.Data)
				// put the pointer in there for other processors to use later in the line
				e.Values["zlib-compressor"] = compressor
				// continue to the next Processor in the decorator stack
				return p.Process(e, task)
			} else {
				return p.Process(e, task)
			}
		})
	}
}
