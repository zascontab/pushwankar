package business

import (
	"context"
	"fmt"
	"time"

	pb "notification-service/pkg/proto"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BusinessClient implementa el cliente para comunicarse con el servicio business
type BusinessClient struct {
	conn          *grpc.ClientConn
	client        pb.BusinessServiceClient
	serverAddress string
}

// NewBusinessClient crea una nueva instancia del cliente
func NewBusinessClient(serverAddress string) (*BusinessClient, error) {
	client := &BusinessClient{
		serverAddress: serverAddress,
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establece la conexión gRPC
func (c *BusinessClient) connect() error {
	conn, err := grpc.Dial(
		c.serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return fmt.Errorf("failed to connect to business service: %w", err)
	}

	c.conn = conn
	c.client = pb.NewBusinessServiceClient(conn)

	return nil
}

// Close cierra la conexión
func (c *BusinessClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ValidateUser verifica si un usuario existe y es válido
func (c *BusinessClient) ValidateUser(ctx context.Context, userID string) (bool, error) {
	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var result bool
	var resp *pb.ValidateUserResponse

	operation := func() error {
		var err error

		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		resp, err = c.client.ValidateUser(reqCtx, &pb.ValidateUserRequest{
			UserId: userID,
		})

		if err != nil {
			return fmt.Errorf("error calling ValidateUser: %w", err)
		}

		result = resp.IsValid
		return nil
	}

	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return false, err
	}

	return result, nil
}

// GetDeviceInfo obtiene información de un dispositivo
func (c *BusinessClient) GetDeviceInfo(ctx context.Context, deviceID string) (*pb.DeviceInfo, error) {
	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var deviceInfo *pb.DeviceInfo
	var resp *pb.GetDeviceInfoResponse

	operation := func() error {
		var err error

		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		resp, err = c.client.GetDeviceInfo(reqCtx, &pb.GetDeviceInfoRequest{
			DeviceId: deviceID,
		})

		if err != nil {
			return fmt.Errorf("error calling GetDeviceInfo: %w", err)
		}

		if !resp.Success {
			return fmt.Errorf("business service error: %s", resp.ErrorMessage)
		}

		deviceInfo = resp.Device
		return nil
	}

	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return nil, err
	}

	return deviceInfo, nil
}

// GetUserDevices obtiene los dispositivos de un usuario
func (c *BusinessClient) GetUserDevices(ctx context.Context, userID string) ([]*pb.DeviceInfo, error) {
	// Configurar backoff para reintentos
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = 10 * time.Second

	var devices []*pb.DeviceInfo
	var resp *pb.GetUserDevicesResponse

	operation := func() error {
		var err error

		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		resp, err = c.client.GetUserDevices(reqCtx, &pb.GetUserDevicesRequest{
			UserId: userID,
		})

		if err != nil {
			return fmt.Errorf("error calling GetUserDevices: %w", err)
		}

		if !resp.Success {
			return fmt.Errorf("business service error: %s", resp.ErrorMessage)
		}

		devices = resp.Devices
		return nil
	}

	err := backoff.Retry(operation, expBackoff)
	if err != nil {
		return nil, err
	}

	return devices, nil
}
