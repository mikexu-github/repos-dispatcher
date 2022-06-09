package restful

import (
	"github.com/quanxiang-cloud/dispatcher/pkg/handout"
	"github.com/quanxiang-cloud/dispatcher/pkg/misc/logger"
	"go.uber.org/zap"
)

type mock struct{}

func (m *mock) Do(param *handout.Param) error {
	// do something
	logger.Logger.Infow("hand out something", zap.String("code", param.Code))
	return nil
}
