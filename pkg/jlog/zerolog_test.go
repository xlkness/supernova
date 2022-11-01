package jlog

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestGlobalAndSubLogger(t *testing.T) {
	l := zerolog.New(os.Stdout).With().Logger()
	l.Log().Str("server", "sdfsdf").Send()

	fh, err := NewFileHandler("log/test.log", os.O_RDWR|os.O_CREATE|os.O_APPEND)
	if err != nil {
		panic(err)
	}
	NewGlobalLogger(fh, LogLevelTrace, func(l zerolog.Logger) zerolog.Logger {
		return l.With().Str("test_key", "test_value").Logger()
	}, true)

	subLogger1 := GetSubLogger().Str("test_sub_key1", "test_sub_value1").Logger()
	subLogger2 := GetSubLogger().Str("test_sub_key2", "test_sub_value2").Logger()

	Debugf("test debug")
	Critif("test criti")
	return

	n := 500
	line := 100
	wg := sync.WaitGroup{}
	wg.Add(n)
	t1 := time.Now()
	for i := 0; i < n; i++ {
		go func() {
			for j := 0; j < line; j++ {
				// 全局日志包含test_key
				log.Debug().Int64("player", 124234234).Err(fmt.Errorf("unexpected eof")).Msg("get player info error")
				// logger1继承全局日志包含test_key，并且包含test_sub_key1
				subLogger1.Debug().Int64("player", 124234234).Err(fmt.Errorf("unexpected eof")).Msg("get player info error")
				// logger2继承全局日志包含test_key，并且包含test_sub_key2
				subLogger2.Debug().Int64("player", 124234234).Int("level", 123).Err(fmt.Errorf("unexpected eof")).Msg("get player info error")
			}
			wg.Done()
		}()
	}

	wg.Wait()
	t2 := time.Since(t1).Milliseconds()
	if int(t2/1000) <= 0 {
		t2 = 1000
	}
	fmt.Printf("over:%d/s\n", (n*line)/int(t2/1000))

	// 设置日志等级
	assertBool(true, log.Trace().Enabled())
	assertBool(true, subLogger1.Trace().Enabled())
	assertBool(true, subLogger2.Trace().Enabled())
	zerolog.SetGlobalLevel(LogLevelDebug)
	assertBool(false, log.Trace().Enabled())
	assertBool(true, log.Debug().Enabled())
	assertBool(true, log.Info().Enabled())
	assertBool(false, subLogger1.Trace().Enabled())
	assertBool(false, subLogger2.Trace().Enabled())
	assertBool(true, subLogger1.Debug().Enabled())
	assertBool(true, subLogger2.Debug().Enabled())

	// 一定要这样设置日志等级
	subLogger1 = subLogger1.Level(LogLevelTrace)
	assertBool(false, subLogger1.Trace().Enabled())
	assertBool(false, log.Trace().Enabled())
	assertBool(false, subLogger2.Trace().Enabled())

	subLogger1 = subLogger1.Level(LogLevelInfo)
	assertBool(false, log.Trace().Enabled())
	assertBool(true, log.Debug().Enabled())
	assertBool(false, subLogger1.Debug().Enabled())
	assertBool(true, subLogger1.Info().Enabled())
	assertBool(true, subLogger2.Debug().Enabled())
	assertBool(false, subLogger2.Trace().Enabled())
}

func assertBool(expected bool, find bool) {
	if expected != find {
		panic(fmt.Errorf("expected:%v, find:%v", expected, find))
	}
}
