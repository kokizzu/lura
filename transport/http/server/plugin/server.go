// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"context"
	"net/http"

	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/logging"
)

const Namespace = "github_com/devopsfaith/krakend/transport/http/server/handler"
const logPrefix = "[PLUGIN: Server]"

type RunServer func(context.Context, config.ServiceConfig, http.Handler) error

func New(logger logging.Logger, next RunServer) RunServer {
	return func(ctx context.Context, cfg config.ServiceConfig, handler http.Handler) error {
		v, ok := cfg.ExtraConfig[Namespace]

		if !ok {
			return next(ctx, cfg, handler)
		}
		extra, ok := v.(map[string]interface{})
		if !ok {
			logger.Debug(logPrefix, "Wrong extra_config type")
			return next(ctx, cfg, handler)
		}

		// load plugin(s)
		r, ok := serverRegister.Get(Namespace)
		if !ok {
			logger.Debug(logPrefix, "No plugins registered for the module")
			return next(ctx, cfg, handler)
		}

		name, nameOk := extra["name"].(string)
		fifoRaw, fifoOk := extra["name"].([]interface{})
		if !nameOk && !fifoOk {
			logger.Debug(logPrefix, "No plugins required in the extra config")
			return next(ctx, cfg, handler)
		}
		var fifo []string

		if !fifoOk {
			fifo = []string{name}
		} else {
			for _, x := range fifoRaw {
				if v, ok := x.(string); ok {
					fifo = append(fifo, v)
				}
			}
		}

		for _, name := range fifo {
			rawHf, ok := r.Get(name)
			if !ok {
				logger.Error(logPrefix, "No plugin registered as", name)
				continue
			}

			hf, ok := rawHf.(func(context.Context, map[string]interface{}, http.Handler) (http.Handler, error))
			if !ok {
				logger.Error(logPrefix, "Wrong plugin handler type:", name)
				continue
			}

			handlerWrapper, err := hf(ctx, extra, handler)
			if err != nil {
				logger.Error(logPrefix, "Error getting the plugin handler:", err.Error())
				continue
			}

			logger.Info(logPrefix, "Injecting plugin", name)
			handler = handlerWrapper
		}
		return next(ctx, cfg, handler)
	}
}
