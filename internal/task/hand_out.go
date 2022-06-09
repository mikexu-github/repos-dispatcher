package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/quanxiang-cloud/dispatcher/internal/models"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
)

type handOut interface {
	HandOut(context.Context, *models.TaskLine) error
}

const (
	handoutURI = "http://%s/api/v1/handout/%s"
)

// HTTPHandOut restful的方式分发
type HTTPHandOut struct {
	conf *Config
}

// Config config
type Config struct {
	Deadline     time.Duration
	DialTimeout  time.Duration
	MaxIdleConns int
}

type handOutReq struct {
	Code string `json:"code"`
}

// NewHTTPHandOut new restful HandOut
func NewHTTPHandOut(conf *Config) *HTTPHandOut {
	return &HTTPHandOut{
		conf: conf,
	}
}

// HandOut 分发
func (h *HTTPHandOut) HandOut(ctx context.Context, taskLine *models.TaskLine) error {
	// TODO 连接池
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				deadline := time.Now().Add(h.conf.Deadline)
				c, err := net.DialTimeout(netw, addr, h.conf.DialTimeout)
				if err != nil {
					return nil, err
				}
				c.SetDeadline(deadline)
				return c, nil
			},
			MaxIdleConns: h.conf.MaxIdleConns,
		},
	}

	jsonBody, err := json.Marshal(handOutReq{
		Code: taskLine.Code,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", genHandOutURL(taskLine.Code), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Add("Context-Type", "application/json")
	req.Header.Add("Requsest-Id", logger.STDRequestID(ctx).String)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("http status [" + resp.Status + "]")
	}
	return nil
}

func genHandOutURL(base string) string {
	param0 := strings.Split(base, ":")[0]
	param1 := strings.Split(base, ":")[1]
	return fmt.Sprintf(handoutURI, param0, param1)
}
