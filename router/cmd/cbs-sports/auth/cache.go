package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type cache struct {
	client redis.Cmdable
	logger *zap.Logger
}

type cachedUser struct {
	Id                 int64            `json:"id"`
	FirstName          string           `json:"firstName"`
	LastName           string           `json:"lastName"`
	PreferredEntryName string           `json:"preferredEntryName"`
	Email              string           `json:"email"`
	EncryptedPid       string           `json:"encryptedPid"`
	CustId             int64            `json:"custId"`
	PSeq               int              `json:"pseq"`
	Extra              *json.RawMessage `json:"extra"`
	UserLogin          string           `json:"userLogin"`
	HasUsedCbsApp      bool             `json:"hasUsedCbsApp,omitempty"`
	AvatarUrl          string           `json:"avatarUrl"`
}

func newCache(logger *zap.Logger) (*cache, error) {
	redisClusterHost, redisClusterPort, redisUsesCluster := "localhost", 6379, false

	if value, exists := os.LookupEnv("USER_REDIS_CLUSTER_HOST"); exists {
		redisClusterHost = value
	} else {
		return nil, fmt.Errorf("USER_REDIS_CLUSTER_HOST env variable no found")
	}

	if value, exists := os.LookupEnv("USER_REDIS_CLUSTER_PORT"); exists {
		if port, err := strconv.Atoi(value); err == nil {
			redisClusterPort = port
		}
	} else {
		return nil, fmt.Errorf("USER_REDIS_CLUSTER_PORT env variable no found")

	}

	if value, exists := os.LookupEnv("USER_REDIS_USES_CLUSTER"); exists {
		redisUsesCluster = strings.EqualFold(value, "true")
	} else {
		return nil, fmt.Errorf("USER_REDIS_USES_CLUSTER env variable no found")

	}

	addr := fmt.Sprintf("%s:%d", redisClusterHost, redisClusterPort)
	if redisUsesCluster {
		return &cache{redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: []string{addr},
		}), logger}, nil
	}

	return &cache{redis.NewClient(&redis.Options{
		Addr: addr,
	}), logger}, nil
}

func (c cache) userKey(userLogin string) string {
	return fmt.Sprintf("SAPIUser:%s", userLogin)
}

func (c cache) exists(ctx context.Context, userLogin string) (bool, error) {
	userKey := c.userKey(userLogin)
	c.logger.Debug("checking cached user", zap.String("key", userKey))

	result, err := c.client.Exists(ctx, userKey).Result()
	if err != nil {
		return false, fmt.Errorf("error while reading cached user: %v", err)
	}

	return result > 0, nil
}

func (c cache) saveUser(ctx context.Context, user cachedUser) error {
	userKey := c.userKey(user.UserLogin)
	c.logger.Debug("saving cached user", zap.String("key", userKey))

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("error while marshaling cached user: %v", err)
	}

	_, err = c.client.SetEx(ctx, userKey, string(data), time.Hour*24*7).Result()
	if err != nil {
		return fmt.Errorf("error while saving cached user: %v", err)
	}

	return nil
}
