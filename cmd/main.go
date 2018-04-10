package main

import (
	"context"
	"encoding/csv"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis"

	"github.com/CityOfZion/neo-go-sdk/neo"
	"github.com/blocksports/block-sports-scheduler"
	"github.com/go-kit/kit/log"
	"github.com/pusher/pusher-http-go"
)

func main() {
	/*
		Set up logging
	*/

	var logger log.Logger
	{
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stdout))
		logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	}
	stdlog.SetOutput(log.NewStdlibAdapter(logger))

	/*
		Create new redis client
	*/

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

	/*
		Enable pusher client
	*/

	pusherClient := pusher.Client{
		AppId:   os.Getenv("PUSHER_ID"),
		Key:     os.Getenv("PUSHER_KEY"),
		Secret:  os.Getenv("PUSHER_SECRET"),
		Cluster: os.Getenv("PUSHER_CLUSTER"),
		Secure:  true,
	}

	httpClient := &http.Client{Timeout: time.Second * 5}
	pusherClient.HttpClient = httpClient

	/*
		Load in node uris and create new Neo client
	*/

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

	/*
		Initialise service
	*/

	svc := service.NewService(logger, redisClient, &pusherClient, neoClient)

	/*
		Create healthcheck web service
	*/

	var ctx context.Context
	{
		ctx = context.Background()
	}

	h := svc.MakeHTTPHandler(ctx, logger)

	serviceAddr := os.Getenv("SERVICE_ADDR")
	if serviceAddr == "" {
		panic("SERVICE_ADDR not found")
	}

	logger.Log("msg", fmt.Sprintf("Listening on port %s", serviceAddr))
	stdlog.Fatal(http.ListenAndServe(serviceAddr, h))

	/*
		shutdown
	*/

	end := shutdown(svc)
	<-end
}

func shutdown(svc *service.Service) <-chan struct{} {
	end := make(chan struct{})
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s
		fmt.Println("Shutting down gracefully.")
		svc.Cron.Stop()
		svc.RedisClient.Close()
		close(end)
	}()
	return end
}
