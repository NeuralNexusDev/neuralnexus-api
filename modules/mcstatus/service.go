package mcstatus

import (
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
)

var (
	ErrJavaStatus    = errors.New("failed to get java server status")
	ErrBedrockStatus = errors.New("failed to get bedrock server status")
)

// MCStatusService - Minecraft Status service
type MCStatusService interface {
	GetJavaServerStatus(host string, port int, queryEnabled bool, queryPort int) (*MCServerStatus, error)
	GetBedrockServerStatus(host string, port int) (*MCServerStatus, error)
	GetServerStatus(host string, port int, isBedrock bool, queryEnabled bool, queryPort int) (*MCServerStatus, error)
}

// service - Minecraft Status service implementation
type service struct{}

// NewService - Create new Minecraft Status service
func NewService() MCStatusService {
	return &service{}
}

func pingJavaStatus(pinger *minequery.Pinger, host string, port int) *MCServerStatus {
	if s17, err := pinger.Ping17(host, port); err == nil {
		return GetPing17Status(s17)
	}
	if s16, err := pinger.Ping16(host, port); err == nil {
		return GetPing16Status(s16)
	}
	if s14, err := pinger.Ping14(host, port); err == nil {
		return GetPing14Status(s14)
	}
	if sb18, err := pinger.PingBeta18(host, port); err == nil {
		return GetBeta18Status(sb18)
	}
	return nil
}

func mergeQueryStatus(ping, query *MCServerStatus) *MCServerStatus {
	query.Icon = ping.Icon
	query.Legacy = ping.Legacy
	query.Favicon = ping.Favicon
	return query
}

// GetJavaServerStatus - Get Java server status
func (s *service) GetJavaServerStatus(host string, port int, queryEnabled bool, queryPort int) (*MCServerStatus, error) {
	pinger := minequery.NewPinger(
		minequery.WithTimeout(5*time.Second),
		minequery.WithProtocolVersion16(minequery.Ping16ProtocolVersion162),
		minequery.WithProtocolVersion17(minequery.Ping17ProtocolVersion119),
	)

	var queryResult chan *minequery.FullQueryStatus
	if queryEnabled {
		queryResult = make(chan *minequery.FullQueryStatus)
		go func() {
			query, _ := pinger.QueryFull(host, queryPort)
			queryResult <- query
		}()
	}

	status := pingJavaStatus(pinger, host, port)
	var query *minequery.FullQueryStatus
	if queryResult != nil {
		query = <-queryResult
	}
	switch {
	case status != nil && query != nil:
		status = mergeQueryStatus(status, GetQueryStatus(query))
	case query != nil:
		status = GetQueryStatus(query)
	case status == nil:
		return nil, ErrJavaStatus
	}
	status.Host = host
	status.Port = int32(port)
	return status, nil
}

func bedrockAddress(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// GetBedrockServerStatus - Get Bedrock server status
func (s *service) GetBedrockServerStatus(host string, port int) (*MCServerStatus, error) {
	status, err := bedrockping.Query(bedrockAddress(host, port), 5*time.Second, 150*time.Millisecond)
	if err != nil {
		log.Println(ErrBedrockStatus, err)
		return nil, ErrBedrockStatus
	}
	bedrockStatus := GetBedrockStatus(status)
	bedrockStatus.Host = host
	bedrockStatus.Port = int32(port)
	return bedrockStatus, nil
}

// GetServerStatus - Get server status
func (s *service) GetServerStatus(host string, port int, isBedrock bool, queryEnabled bool, queryPort int) (*MCServerStatus, error) {
	if isBedrock {
		return s.GetBedrockServerStatus(host, port)
	}
	return s.GetJavaServerStatus(host, port, queryEnabled, queryPort)
}
