package logger

import (
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config defines the config for logger middleware
type Config struct {
	Logger *zerolog.Logger
	// UTC a boolean stating whether to use UTC time zone or local.
	UTC            bool
	SkipPath       []string
	SkipPathRegexp *regexp.Regexp

	// Additional context fields to be logged. The functions must return 2
	// values: the key and the value for the log context.
	ContextFieldFuns []func(*gin.Context) (string, string)
}

// SetLogger initializes the logging middleware.
func SetLogger(config ...Config) gin.HandlerFunc {
	var newConfig Config
	if len(config) > 0 {
		newConfig = config[0]
	}
	var skip map[string]struct{}
	if length := len(newConfig.SkipPath); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range newConfig.SkipPath {
			skip[path] = struct{}{}
		}
	}

	var sublog zerolog.Logger
	if newConfig.Logger == nil {
		sublog = log.Logger
	} else {
		sublog = *newConfig.Logger
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}

		c.Next()
		track := true

		if _, ok := skip[path]; ok {
			track = false
		}

		if track &&
			newConfig.SkipPathRegexp != nil &&
			newConfig.SkipPathRegexp.MatchString(path) {
			track = false
		}

		if track {
			end := time.Now()
			latency := end.Sub(start)
			if newConfig.UTC {
				end = end.UTC()
			}

			msg := "Request"
			if len(c.Errors) > 0 {
				msg = c.Errors.String()
			}

			subCtx := sublog.With().
				Int("status", c.Writer.Status()).
				Str("method", c.Request.Method).
				Str("path", path).
				Str("ip", c.ClientIP()).
				Dur("latency", latency).
				Str("user-agent", c.Request.UserAgent())
			for _, f := range newConfig.ContextFieldFuns {
				subCtx = subCtx.Str(f(c))
			}
			dumplogger := subCtx.Logger()

			switch {
			case c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError:
				{
					dumplogger.Warn().
						Msg(msg)
				}
			case c.Writer.Status() >= http.StatusInternalServerError:
				{
					dumplogger.Error().
						Msg(msg)
				}
			default:
				dumplogger.Info().
					Msg(msg)
			}
		}

	}
}
