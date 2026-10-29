package collector

import (
	"net"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	AF_INET  = 2
	AF_INET6 = 23

	TCP_TABLE_OWNER_PID_ALL = 5
	UDP_TABLE_OWNER_PID     = 1

	MIB_TCP_STATE_CLOSED      = 1
	MIB_TCP_STATE_LISTEN      = 2
	MIB_TCP_STATE_SYN_SENT    = 3
	MIB_TCP_STATE_SYN_RCVD    = 4
	MIB_TCP_STATE_ESTAB       = 5
	MIB_TCP_STATE_FIN_WAIT1   = 6
	MIB_TCP_STATE_FIN_WAIT2   = 7
	MIB_TCP_STATE_CLOSE_WAIT  = 8
	MIB_TCP_STATE_CLOSING     = 9
	MIB_TCP_STATE_LAST_ACK    = 10
	MIB_TCP_STATE_TIME_WAIT   = 11
	MIB_TCP_STATE_DELETE_TCB  = 12
)

type Connection struct {
	PID         uint32
	LocalIP     string
	LocalPort   uint16
	RemoteIP    string
	RemotePort  uint16
	State       uint32
	Protocol    string
}

type mibTcpRowOwnerPid struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPid  uint32
}

type mibTcpTableOwnerPid struct {
	NumEntries uint32
	Table      [1]mibTcpRowOwnerPid
}

type mibUdpRowOwnerPid struct {
	LocalAddr  uint32
	LocalPort  uint32
	OwningPid  uint32
}

type mibUdpTableOwnerPid struct {
	NumEntries uint32
	Table      [1]mibUdpRowOwnerPid
}

type mibTcp6RowOwnerPid struct {
	LocalAddr  [16]byte
	LocalPort  uint32
	RemoteAddr [16]byte
	RemotePort uint32
	State      uint32
	OwningPid  uint32
}

type mibTcp6TableOwnerPid struct {
	NumEntries uint32
	Table      [1]mibTcp6RowOwnerPid
}

type mibUdp6RowOwnerPid struct {
	LocalAddr  [16]byte
	LocalPort  uint32
	OwningPid  uint32
}

type mibUdp6TableOwnerPid struct {
	NumEntries uint32
	Table      [1]mibUdp6RowOwnerPid
}

var (
	iphlpapi               = windows.NewLazySystemDLL("iphlpapi.dll")
	getExtendedTcpTable    = iphlpapi.NewProc("GetExtendedTcpTable")
	getExtendedUdpTable    = iphlpapi.NewProc("GetExtendedUdpTable")
)

func GetTcpConnections() ([]Connection, error) {
	var conns []Connection

	conns = append(conns, getTcp4Connections()...)
	conns = append(conns, getTcp6Connections()...)

	return conns, nil
}

func GetUdpConnections() ([]Connection, error) {
	var conns []Connection

	conns = append(conns, getUdp4Connections()...)
	conns = append(conns, getUdp6Connections()...)

	return conns, nil
}

func getTcp4Connections() []Connection {
	var conns []Connection

	size := uint32(0)
	ret, _, _ := getExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 1, AF_INET, TCP_TABLE_OWNER_PID_ALL, 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return conns
	}

	buf := make([]byte, size)
	ret, _, _ = getExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, AF_INET, TCP_TABLE_OWNER_PID_ALL, 0)
	if ret != 0 {
		return conns
	}

	table := (*mibTcpTableOwnerPid)(unsafe.Pointer(&buf[0]))
	count := int(table.NumEntries)

	slice := unsafe.Slice(&table.Table[0], count)
	for _, row := range slice {
		if row.OwningPid == 0 || row.OwningPid == 4 {
			continue
		}

		localIP := intToIP4(row.LocalAddr)
		remoteIP := intToIP4(row.RemoteAddr)
		localPort := uint16(ntohs(row.LocalPort))
		remotePort := uint16(ntohs(row.RemotePort))

		if isLoopbackOrUnspecified(remoteIP) {
			continue
		}

		conns = append(conns, Connection{
			PID:        row.OwningPid,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      row.State,
			Protocol:   "tcp",
		})
	}

	runtime.KeepAlive(buf)
	return conns
}

func getTcp6Connections() []Connection {
	var conns []Connection

	size := uint32(0)
	ret, _, _ := getExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 1, AF_INET6, TCP_TABLE_OWNER_PID_ALL, 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return conns
	}

	buf := make([]byte, size)
	ret, _, _ = getExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, AF_INET6, TCP_TABLE_OWNER_PID_ALL, 0)
	if ret != 0 {
		return conns
	}

	table := (*mibTcp6TableOwnerPid)(unsafe.Pointer(&buf[0]))
	count := int(table.NumEntries)

	slice := unsafe.Slice(&table.Table[0], count)
	for _, row := range slice {
		if row.OwningPid == 0 || row.OwningPid == 4 {
			continue
		}

		localIP := intToIP6(row.LocalAddr[:])
		remoteIP := intToIP6(row.RemoteAddr[:])
		localPort := uint16(ntohs(row.LocalPort))
		remotePort := uint16(ntohs(row.RemotePort))

		if isLoopbackOrUnspecified(remoteIP) {
			continue
		}

		conns = append(conns, Connection{
			PID:        row.OwningPid,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      row.State,
			Protocol:   "tcp",
		})
	}

	runtime.KeepAlive(buf)
	return conns
}

func getUdp4Connections() []Connection {
	var conns []Connection

	size := uint32(0)
	ret, _, _ := getExtendedUdpTable.Call(0, uintptr(unsafe.Pointer(&size)), 1, AF_INET, UDP_TABLE_OWNER_PID, 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return conns
	}

	buf := make([]byte, size)
	ret, _, _ = getExtendedUdpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, AF_INET, UDP_TABLE_OWNER_PID, 0)
	if ret != 0 {
		return conns
	}

	table := (*mibUdpTableOwnerPid)(unsafe.Pointer(&buf[0]))
	count := int(table.NumEntries)

	slice := unsafe.Slice(&table.Table[0], count)
	for _, row := range slice {
		if row.OwningPid == 0 || row.OwningPid == 4 {
			continue
		}

		localIP := intToIP4(row.LocalAddr)
		localPort := uint16(ntohs(row.LocalPort))

		conns = append(conns, Connection{
			PID:        row.OwningPid,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   "",
			RemotePort: 0,
			State:      0,
			Protocol:   "udp",
		})
	}

	runtime.KeepAlive(buf)
	return conns
}

func getUdp6Connections() []Connection {
	var conns []Connection

	size := uint32(0)
	ret, _, _ := getExtendedUdpTable.Call(0, uintptr(unsafe.Pointer(&size)), 1, AF_INET6, UDP_TABLE_OWNER_PID, 0)
	if ret != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return conns
	}

	buf := make([]byte, size)
	ret, _, _ = getExtendedUdpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 1, AF_INET6, UDP_TABLE_OWNER_PID, 0)
	if ret != 0 {
		return conns
	}

	table := (*mibUdp6TableOwnerPid)(unsafe.Pointer(&buf[0]))
	count := int(table.NumEntries)

	slice := unsafe.Slice(&table.Table[0], count)
	for _, row := range slice {
		if row.OwningPid == 0 || row.OwningPid == 4 {
			continue
		}

		localIP := intToIP6(row.LocalAddr[:])
		localPort := uint16(ntohs(row.LocalPort))

		conns = append(conns, Connection{
			PID:        row.OwningPid,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   "",
			RemotePort: 0,
			State:      0,
			Protocol:   "udp",
		})
	}

	runtime.KeepAlive(buf)
	return conns
}

func intToIP4(addr uint32) string {
	return net.IPv4(byte(addr), byte(addr>>8), byte(addr>>16), byte(addr>>24)).String()
}

func intToIP6(addr []byte) string {
	ip := make(net.IP, 16)
	copy(ip, addr)
	return ip.String()
}

func ntohs(port uint32) uint32 {
	return (port >> 8) & 0xFF | (port & 0xFF) << 8
}

func isLoopbackOrUnspecified(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "0.0.0.0" || ip == "::"
}