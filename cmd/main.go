package main

import (
	"encoding/csv"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron"

	"github.com/go-redis/redis"

	"github.com/CityOfZion/neo-go-sdk/neo"
	"github.com/blocksports/block-sports-scheduler"
	"github.com/go-kit/kit/log"
	"github.com/pusher/pusher-http-go"
)

func main() {
	var logger log.Logger
	{
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
		logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	}
	stdlog.SetOutput(log.NewStdlibAdapter(logger))

	redisAddr := os.Getenv("REDIS_ADDR")

	redisClient := redis.NewClient(&redis.Options{
		Addr:        redisAddr,
		Password:    "",
		IdleTimeout: 5 * time.Minute,
		MaxRetries:  3,
	})

	_, err := redisClient.Ping().Result()
	if err != nil {
		fmt.Println(err)
		return
	}

	pusherClient := pusher.Client{
		AppId:   os.Getenv("PUSHER_ID"),
		Key:     os.Getenv("PUSHER_KEY"),
		Secret:  os.Getenv("PUSHER_SECRET"),
		Cluster: os.Getenv("PUSHER_CLUSTER"),
		Secure:  true,
	}

	httpClient := &http.Client{Timeout: time.Second * 5}
	pusherClient.HttpClient = httpClient

	file, err := os.Open("node_uris.csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	csvR := csv.NewReader(file)
	nodeURIs, err := csvR.Read()
	if err != nil {
		fmt.Println(err)
		return
	}

	neoClient, err := neo.NewClientUsingMultipleNodes(nodeURIs)
	if err != nil {
		fmt.Println(err)
		return
	}

	svc := service.NewService(logger, redisClient, &pusherClient, neoClient)

	cron := svc.InitialiseScheduler()

	end := shutdown(cron)
	<-end
}

func shutdown(cron *cron.Cron) <-chan struct{} {
	end := make(chan struct{})
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		fmt.Println("Shutting down gracefully.")
		cron.Stop()
		close(end)
	}()
	return end
}
