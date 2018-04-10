package service

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/CityOfZion/neo-go-sdk/neo"
	"github.com/go-kit/kit/log"
	"github.com/go-redis/redis"
	pusher "github.com/pusher/pusher-http-go"
)

type Service struct {
	Logger       log.Logger
	RedisClient  *redis.Client
	PusherClient *pusher.Client
	NeoClient    *neo.Client
	Internals    InternalDetails
}

type InternalDetails struct {
	UpdatedAt     time.Time
	BlockHeight   int64
	BlocksCounted int64
	DebugCount    int64
	AverageTime   float64
	TimeCounted   float64
	PriceDetails  PriceData
	SportKeys     []SportKey
}

// NewService prepares a new scheduler service
func NewService(logger log.Logger, redisClient *redis.Client, pusherClient *pusher.Client, neoClient *neo.Client) *Service {
	return &Service{
		Logger:       logger,
		RedisClient:  redisClient,
		PusherClient: pusherClient,
		NeoClient:    neoClient,
		Internals: InternalDetails{
			BlockHeight:   0,
			TimeCounted:   0,
			BlocksCounted: 1,
			UpdatedAt:     time.Now(),
		},
	}
}

type BlockchainData struct {
	BlockHeight      int64     `json:"block_height"`
	AverageBlockTime float64   `json:"average_time"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PriceData struct {
	CurrencyData map[string]Currency
	ExchangeRate float64
}

type Currency map[string]float64

func (svc *Service) UpdateBlockHeight(height int64) error {
	now := time.Now()

	timeDifference := now.Sub(svc.Internals.UpdatedAt).Seconds()
	averageTime := svc.Internals.TimeCounted / float64(svc.Internals.BlocksCounted)

	svc.Internals.TimeCounted += timeDifference
	svc.Internals.BlockHeight = height
	svc.Internals.UpdatedAt = now
	svc.Internals.AverageTime = averageTime
	svc.Internals.BlocksCounted++

	data := BlockchainData{
		BlockHeight:      height,
		AverageBlockTime: averageTime,
		UpdatedAt:        now,
	}

	return svc.SetRedis("blockchain_data", &data)
}

func (svc *Service) GetRedis(key string, v interface{}) (err error) {
	interfaceRaw, err := svc.RedisClient.Get(key).Bytes()
	if err != nil {
		return
	}

	return json.Unmarshal(interfaceRaw, v)
}

func (svc *Service) SetRedis(key string, v interface{}) (err error) {
	interfaceJSON, err := json.Marshal(v)
	if err != nil {
		return
	}

	return svc.RedisClient.Set(key, interfaceJSON, 0).Err()
}

func (svc *Service) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode("OK")
}
