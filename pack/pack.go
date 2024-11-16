package pack

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type Package struct {
	Length int32
	Type   int32
	Data   []byte
}

const (
	TYPE_PORTS = iota
	TYPE_HEARTBEAT
	TYPE_DATA
	TYPE_VISITOR
	TYPE_VISITOR_NEW
	TYPE_VISITOR_CLOSE
	PACK_HEAD_LENGTH   = 8
	PACK_MAX_SIZE      = 1024 * 1024
	HEARTBEAT_INTERVAL = 30
	HEARTBEAT_TOLERATE = 3
)

func Pack(data []byte, t int) ([]byte, error) {
	buffer := bytes.NewBuffer([]byte{})

	if err := binary.Write(buffer, binary.LittleEndian, int32(len(data))); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.LittleEndian, int32(t)); err != nil {
		return nil, err
	}

	if err := binary.Write(buffer, binary.LittleEndian, data); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func ReadPackage(conn net.Conn) (*Package, error) {
	headBuf := bytes.NewBuffer(make([]byte, PACK_HEAD_LENGTH))
	if _, err := io.ReadFull(conn, headBuf.Bytes()); err != nil {
		return nil, err
	}

	// now we get head info
	pkg := &Package{}
	// read length
	if err := binary.Read(headBuf, binary.LittleEndian, &pkg.Length); err != nil {
		return nil, err
	}
	// read type
	if err := binary.Read(headBuf, binary.LittleEndian, &pkg.Type); err != nil {
		return nil, err
	}
	// read data
	if pkg.Length > 0 {
		pkg.Data = make([]byte, pkg.Length)
		if _, err := io.ReadFull(conn, pkg.Data); err != nil {
			return nil, err
		}
	}

	return pkg, nil
}

func SendData(conn net.Conn, data []byte, t int) error {
	pkg, err := Pack(data, t)
	if err != nil {
		return err
	}

	_, err = conn.Write(pkg)
	return err
}

func SendPickyData(conn net.Conn, data []byte, visitorId, t int) error {
	pkg1, e1 := Pack([]byte(fmt.Sprintf("%d", visitorId)), TYPE_VISITOR)
	if e1 != nil {
		return e1
	}
	pkg2, e2 := Pack(data, t)
	if e2 != nil {
		return e2
	}

	pkg := append(pkg1, pkg2...)
	_, err := conn.Write(pkg)
	return err
}
