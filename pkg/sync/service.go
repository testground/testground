package sync

import (
	"github.com/go-redis/redis/v7"
	"go.uber.org/zap"
)

type DefaultService struct {
	rclient *redis.Client
	log     *zap.SugaredLogger
}

func NewService() Service {
	// TODO
	return &DefaultService{}
}

