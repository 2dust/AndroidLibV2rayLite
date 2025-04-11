package libv2ray

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	v2net "github.com/v2fly/v2ray-core/v5/common/net"
	v2internet "github.com/v2fly/v2ray-core/v5/transport/internet"
)

type protectSet interface {
	Protect(int) bool
}

type resolved struct {
	domain       string
	IPs          []net.IP
	Port         int
	lastResolved time.Time
	ipIdx        uint8
	ipLock       sync.Mutex
	lastSwitched time.Time
}

// NextIP switches to another resolved IP.
func (r *resolved) NextIP() {
	r.ipLock.Lock()
	defer r.ipLock.Unlock()

	if len(r.IPs) > 1 {
		now := time.Now()
		if now.Sub(r.lastSwitched) < 5*time.Second {
			log.Println("Switching IP too quickly")
			return
		}
		r.lastSwitched = now
		r.ipIdx++
	}

	if r.ipIdx >= uint8(len(r.IPs)) {
		r.ipIdx = 0
	}

	log.Printf("Switched to next IP: %v", r.IPs[r.ipIdx])
}

func (r *resolved) currentIP() net.IP {
	r.ipLock.Lock()
	defer r.ipLock.Unlock()
	if len(r.IPs) > 0 {
		return r.IPs[r.ipIdx]
	}
	return nil
}

// NewProtectedDialer creates a new ProtectedDialer instance.
func NewProtectedDialer(p protectSet) *ProtectedDialer {
	return &ProtectedDialer{
		resolver:   &net.Resolver{PreferGo: false},
		protectSet: p,
	}
}

// ProtectedDialer handles protected dialing.
type ProtectedDialer struct {
	currentServer string
	resolveChan   chan struct{}
	preferIPv6    bool

	vServer  *resolved
	resolver *net.Resolver

	protectSet
}

func (d *ProtectedDialer) IsVServerReady() bool {
	return d.vServer != nil
}

func (d *ProtectedDialer) PrepareResolveChan() {
	d.resolveChan = make(chan struct{})
}

func (d *ProtectedDialer) ResolveChan() chan struct{} {
	return d.resolveChan
}

// lookupAddr resolves a domain name into IP addresses.
func (d *ProtectedDialer) lookupAddr(addr string) (*resolved, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("SplitHostPort error: %v", err)
		return nil, err
	}

	portnum, err := d.resolver.LookupPort(ctx, "tcp", port)
	if err != nil {
		log.Printf("LookupPort error: %v", err)
		return nil, err
	}

	addrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("failed to resolve domain %s: %v", addr, err)
	}

	IPs := make([]net.IP, 0)
	if d.preferIPv6 {
		for _, ia := range addrs {
			if ia.IP.To4() == nil {
				IPs = append(IPs, ia.IP)
			}
		}
	}
	for _, ia := range addrs {
		if ia.IP.To4() != nil {
			IPs = append(IPs, ia.IP)
		}
	}
	if !d.preferIPv6 {
		for _, ia := range addrs {
			if ia.IP.To4() == nil {
				IPs = append(IPs, ia.IP)
			}
		}
	}

	return &resolved{
		domain:       host,
		IPs:          IPs,
		Port:         portnum,
		lastResolved: time.Now(),
	}, nil
}

// PrepareDomain resolves and caches a domain.
func (d *ProtectedDialer) PrepareDomain(domainName string, closeCh <-chan struct{}, prefIPv6 bool) {
	log.Printf("Preparing Domain: %s", domainName)
	d.currentServer = domainName
	d.preferIPv6 = prefIPv6

	maxRetry := 10
	for {
		if maxRetry == 0 {
			log.Println("Max retries reached for PrepareDomain")
			return
		}

		resolved, err := d.lookupAddr(domainName)
		if err != nil {
			maxRetry--
			log.Printf("PrepareDomain error: %v", err)
			select {
			case <-closeCh:
				log.Println("PrepareDomain exiting due to closure")
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		d.vServer = resolved
		log.Printf("Resolved Domain: %s, Port: %d, IPs: %v",
			resolved.domain, resolved.Port, resolved.IPs)
		return
	}
}

func (d *ProtectedDialer) getFd(network v2net.Network) (fd int, err error) {
	switch network {
	case v2net.Network_TCP:
		fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_STREAM, unix.IPPROTO_TCP)
	case v2net.Network_UDP:
		fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	default:
		err = errors.New("unknown network")
	}
	return
}

// Dial establishes a connection to the destination.
func (d *ProtectedDialer) Dial(ctx context.Context, src v2net.Address, dest v2net.Destination, sockopt *v2internet.SocketConfig) (net.Conn, error) {
	Address := dest.NetAddr()

	if Address == d.currentServer {
		if d.vServer == nil {
			log.Println("Dial pending prepare...")
			<-d.resolveChan
			if d.vServer == nil {
				return nil, fmt.Errorf("failed to prepare domain %s", d.currentServer)
			}
		}

		fd, err := d.getFd(dest.Network)
		if err != nil {
			return nil, err
		}

		curIP := d.vServer.currentIP()
		conn, err := d.fdConn(ctx, curIP, d.vServer.Port, dest.Network, fd)
		if err != nil {
			d.vServer.NextIP()
			return nil, err
		}
		log.Printf("Using Prepared IP: %s", curIP)
		return conn, nil
	}

	resolved, err := d.lookupAddr(Address)
	if err != nil {
		return nil, err
	}

	fd, err := d.getFd(dest.Network)
	if err != nil {
		return nil, err
	}

	return d.fdConn(ctx, resolved.IPs[0], resolved.Port, dest.Network, fd)
}

func (d *ProtectedDialer) fdConn(ctx context.Context, ip net.IP, port int, network v2net.Network, fd int) (net.Conn, error) {
	defer unix.Close(fd)

	if !d.Protect(fd) {
		log.Printf("Failed to protect fd: %d", fd)
		return nil, errors.New("failed to protect socket")
	}

	sa := &unix.SockaddrInet6{Port: port}
	copy(sa.Addr[:], ip.To16())

	if network == v2net.Network_UDP {
		if err := unix.Bind(fd, &unix.SockaddrInet6{}); err != nil {
			return nil, err
		}
	} else {
		if err := unix.Connect(fd, sa); err != nil {
			return nil, err
		}
	}

	file := os.NewFile(uintptr(fd), "Socket")
	if file == nil {
		return nil, errors.New("invalid file descriptor")
	}
	defer file.Close()

	if network == v2net.Network_UDP {
		packetConn, err := net.FilePacketConn(file)
		if err != nil {
			return nil, err
		}
		return &PacketConnWrapper{
			Conn: packetConn,
			Dest: &net.UDPAddr{IP: ip, Port: port},
		}, nil
	}

	conn, err := net.FileConn(file)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// خطوط 300 به بعد بدون تغییر باقی مانده‌اند.
type PacketConnWrapper struct {
	Conn net.PacketConn
	Dest net.Addr
}

func (c *PacketConnWrapper) Close() error {
	return c.Conn.Close()
}

func (c *PacketConnWrapper) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *PacketConnWrapper) RemoteAddr() net.Addr {
	return c.Dest
}

func (c *PacketConnWrapper) Write(p []byte) (int, error) {
	return c.Conn.WriteTo(p, c.Dest)
}

func (c *PacketConnWrapper) Read(p []byte) (int, error) {
	n, _, err := c.Conn.ReadFrom(p)
	return n, err
}

func (c *PacketConnWrapper) WriteTo(p []byte, d net.Addr) (int, error) {
	return c.Conn.WriteTo(p, d)
}

func (c *PacketConnWrapper) ReadFrom(p []byte) (int, net.Addr, error) {
	return c.Conn.ReadFrom(p)
}

func (c *PacketConnWrapper) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *PacketConnWrapper) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *PacketConnWrapper) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}
