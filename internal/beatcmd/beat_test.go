package beatcmd

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common/reload"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/monitoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestRunMaxProcs ensures Beat.Run calls the GOMAXPROCS adjustment code by looking for log messages.
func TestRunMaxProcs(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("%d_GOMAXPROCS", n), func(t *testing.T) {
			t.Setenv("GOMAXPROCS", strconv.Itoa(n))
			beat, _ := newNopBeat(t, "output.console.enabled: true")

			// Capture logs for testing.
			logp.DevelopmentSetup(logp.ToObserverOutput())
			logs := logp.ObserverLogs()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g, ctx := errgroup.WithContext(ctx)
			defer g.Wait()
			g.Go(func() error { return beat.Run(ctx) })

			timeout := time.NewTimer(10 * time.Second)
			defer timeout.Stop()
			for {
				select {
				case <-timeout.C:
					t.Error("timed out waiting for log message, total logs observed:", logs.Len())
					for _, log := range logs.All() {
						t.Log(log.LoggerName, log.Message)
					}
					return
				case <-time.After(10 * time.Millisecond):
				}

				logs := logs.FilterMessageSnippet(fmt.Sprintf(
					`maxprocs: Honoring GOMAXPROCS="%d" as set in environment`, n,
				))
				if logs.Len() > 0 {
					break
				}
			}

			cancel()
			assert.NoError(t, g.Wait())
		})
	}
}

func newNopBeat(t testing.TB, configYAML string) (*Beat, *nopBeater) {
	resetGlobals()
	initCfgfile(t, configYAML)
	nopBeater := newNopBeater()
	beat, err := NewBeat(BeatParams{
		Create: func(b *beat.Beat, cfg *config.C) (beat.Beater, error) {
			return nopBeater, nil
		},
	})
	require.NoError(t, err)
	return beat, nopBeater
}

func resetGlobals() {
	// Clear monitoring registries to allow the new Beat to populate them.
	monitoring.GetNamespace("info").SetRegistry(nil)
	monitoring.GetNamespace("state").SetRegistry(nil)
	for _, name := range []string{"system", "beat", "libbeat"} {
		registry := monitoring.Default.GetRegistry(name)
		if registry != nil {
			registry.Clear()
		}
	}

	// Create a new reload registry, as the Beat.Run method will register with it.
	reload.Register = reload.NewRegistry()
}

type nopBeater struct {
	running chan struct{}
	done    chan struct{}
}

func newNopBeater() *nopBeater {
	return &nopBeater{
		running: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (b *nopBeater) Run(*beat.Beat) error {
	close(b.running)
	<-b.done
	return nil
}

func (b *nopBeater) Stop() {
	close(b.done)
}
