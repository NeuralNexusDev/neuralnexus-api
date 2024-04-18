package mcstatus

import (
	"errors"
	"fmt"
	"time"

	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
)

// MCStatusService - Minecraft Status service
type MCStatusService interface {
	GetJavaServerStatus(host string, port int, queryEnabled bool) (*MCStatusResponse, error)
	GetBedrockServerStatus(host string, port int) (*MCStatusResponse, error)
	GetServerStatus(host string, port int, isBedrock bool, queryEnabled bool) (*MCStatusResponse, error)
}

// service - Minecraft Status service implementation
type service struct{}

// NewService - Create new Minecraft Status service
func NewService() *service {
	return &service{}
}

// GetJavaServerStatus - Get Java server status
func (s *service) GetJavaServerStatus(host string, port int, queryEnabled bool) (*MCStatusResponse, error) {
	pinger := minequery.NewPinger(
		minequery.WithTimeout(5*time.Second),
		minequery.WithProtocolVersion16(minequery.Ping16ProtocolVersion162),
		minequery.WithProtocolVersion17(minequery.Ping17ProtocolVersion119),
	)

	var status *MCStatusResponse = nil
	s17, err := pinger.Ping17(host, port)
	if err == nil {
		status = GetPing17Status(s17)
	}
	s16, err := pinger.Ping16(host, port)
	if err == nil {
		status = GetPing16Status(s16)
	}
	s14, err := pinger.Ping14(host, port)
	if err == nil {
		status = GetPing14Status(s14)
	}
	sb18, err := pinger.PingBeta18(host, port)
	if err == nil {
		status = GetBeta18Status(sb18)
	}

	if queryEnabled {
		query, err := pinger.QueryFull(host, port)
		if err == nil {
			queryStatus := GetQueryStatus(query)
			if status != nil {
				queryStatus.Icon = status.Icon
				status = queryStatus
			}
		}
	}
	if status != nil {
		status.Host = host
		status.Port = port
		return status, nil
	}
	return nil, errors.New("failed to get java server status")
}

// GetBedrockServerStatus - Get Bedrock server status
func (s *service) GetBedrockServerStatus(host string, port int) (*MCStatusResponse, error) {
	connect := host + ":" + fmt.Sprint(port)
	status, err := bedrockping.Query(connect, 5*time.Second, 150*time.Millisecond)
	if err != nil {
		return nil, errors.New("failed to get bedrock server status")
	}
	return GetBedrockStatus(status), nil
}

// GetServerStatus - Get server status
func (s *service) GetServerStatus(host string, port int, isBedrock bool, queryEnabled bool) (*MCStatusResponse, error) {
	if isBedrock {
		return s.GetBedrockServerStatus(host, port)
	}
	return s.GetJavaServerStatus(host, port, queryEnabled)
}
