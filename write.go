package clickhouse

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

func writeBool(conn io.Writer, v bool) error {
	value := []byte{0}
	if v {
		value[0] = 1
	}
	if _, err := conn.Write(value); err != nil {
		return err
	}
	return nil
}

func writeUvarint(conn io.Writer, v uint64) error {
	var (
		buf = make([]byte, binary.MaxVarintLen64)
		len = binary.PutUvarint(buf, v)
	)
	if _, err := conn.Write(buf[0:len]); err != nil {
		return err
	}
	return nil
}

func writeString(conn io.Writer, str string) error {
	if err := writeUvarint(conn, uint64(len([]byte(str)))); err != nil {
		return err
	}
	if _, err := conn.Write([]byte(str)); err != nil {
		return err
	}
	return nil
}

func writeInt32(conn io.Writer, v int32) error {
	if err := binary.Write(conn, binary.LittleEndian, v); err != nil {
		return err
	}
	return nil
}

func writeUInt64(conn io.Writer, v uint64) error {
	if err := binary.Write(conn, binary.LittleEndian, v); err != nil {
		return err
	}
	return nil
}

func write(buffer io.Writer, columnInfo interface{}, v driver.Value) error {
	var err error
	switch columnInfo.(type) {
	case Date:
		var tv time.Time
		switch value := v.(type) {
		case time.Time:
			tv = value
		case string:
			if tv, err = time.Parse("2006-01-02", value); err != nil {
				return err
			}
			tv = time.Date(
				time.Time(tv).Year(),
				time.Time(tv).Month(),
				time.Time(tv).Day(),
				0, 0, 0, 0, time.UTC,
			)
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(tv.Unix()/24/3600))
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	case DateTime:
		var tv time.Time
		switch value := v.(type) {
		case time.Time:
			tv = value
		case string:
			if tv, err = time.Parse("2006-01-02 15:04:05", value); err != nil {
				return err
			}
			tv = time.Date(
				time.Time(tv).Year(),
				time.Time(tv).Month(),
				time.Time(tv).Day(),
				time.Time(tv).Hour(),
				time.Time(tv).Minute(),
				time.Time(tv).Second(),
				0,
				time.UTC,
			)
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(tv.Unix()))
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	case string:
		switch v := v.(type) {
		case []byte:
			var (
				scratch = make([]byte, binary.MaxVarintLen64)
				vlen    = binary.PutUvarint(scratch, uint64(len(v)))
			)
			if _, err := buffer.Write(scratch[0:vlen]); err != nil {
				return err
			}
			if _, err := buffer.Write(v); err != nil {
				return err
			}
		case string:
			scratch := make([]byte, binary.MaxVarintLen64)
			vlen := binary.PutUvarint(scratch, uint64(len(v)))
			if _, err := buffer.Write(scratch[0:vlen]); err != nil {
				return err
			}
			if _, err := buffer.Write([]byte(v)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
	case []byte:
		var (
			strlen int = len(columnInfo.([]byte))
			str    []byte
		)
		switch v := v.(type) {
		case []byte:
			str = v
		case string:
			str = []byte(v)
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		if len(str) > strlen {
			return fmt.Errorf("too large value")
		} else if len(str) == 0 {
			// When empty, insert default value to avoid allocation
			str = columnInfo.([]byte)
		} else if len(str) < strlen {
			fixedString := make([]byte, strlen)
			copy(fixedString, str)
			str = fixedString
		}
		if _, err := buffer.Write(str); err != nil {
			return err
		}
	case float32:
		buf := make([]byte, 4)
		switch value := v.(type) {
		case float32:
			binary.LittleEndian.PutUint32(buf, math.Float32bits(value))
		case float64: // Implicit driver.Value coercion
			binary.LittleEndian.PutUint32(buf, math.Float32bits(float32(value)))
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	case float64:
		switch value := v.(type) {
		case float64:
			buf := make([]byte, 8)
			binary.LittleEndian.PutUint64(buf, math.Float64bits(value))
			if _, err := buffer.Write(buf); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
	case int8, uint8:
		switch value := v.(type) {
		case int8:
			buffer.Write([]byte{uint8(value)})
		case uint8:
			buffer.Write([]byte{value})
		case int64: // Implicit driver.Value coercion
			buffer.Write([]byte{uint8(value)})
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
	case int16, uint16:
		buf := make([]byte, 2)
		switch value := v.(type) {
		case int16:
			binary.LittleEndian.PutUint16(buf, uint16(value))
		case uint16:
			binary.LittleEndian.PutUint16(buf, value)
		case int64: // Implicit driver.Value coercion
			binary.LittleEndian.PutUint16(buf, uint16(value))
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	case int32, uint32:
		buf := make([]byte, 4)
		switch value := v.(type) {
		case int32:
			binary.LittleEndian.PutUint32(buf, uint32(value))
		case uint32:
			binary.LittleEndian.PutUint32(buf, value)
		case int64: // Implicit driver.Value coercion
			binary.LittleEndian.PutUint32(buf, uint32(value))
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	case int64, uint64:
		buf := make([]byte, 8)
		switch value := v.(type) {
		case int64:
			binary.LittleEndian.PutUint64(buf, uint64(value))
		case uint64:
			binary.LittleEndian.PutUint64(buf, value)
		default:
			return fmt.Errorf("unexpected type %T", v)
		}
		if _, err := buffer.Write(buf); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unhandled type %T", v)
	}
	return nil
}
