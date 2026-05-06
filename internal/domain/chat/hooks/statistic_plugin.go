package hooks

import (
	"context"
	"sync"
	"time"

	log "xiaozhi-esp32-server-golang/logger"
)

type turnMetric struct {
	turnID int64

	turnStartTs     int64
	turnEndTs       int64
	asrFirstTextTs  int64
	asrFinalTextTs  int64
	llmStartTs      int64
	llmFirstTokenTs int64
	llmEndTs        int64
	ttsStartTs      int64
	ttsFirstFrameTs int64
	ttsStopTs       int64
}

type statisticPlugin struct {
	mu sync.Mutex

	nextTurn map[string]int64
	current  map[string]*turnMetric
	lastSeen map[string]int64

	cleanupCounter   int64
	cleanupThreshold int64
}

func newStatisticPlugin() *statisticPlugin {
	return &statisticPlugin{
		nextTurn:         make(map[string]int64),
		current:          make(map[string]*turnMetric),
		lastSeen:         make(map[string]int64),
		cleanupThreshold: 100,
	}
}

func (p *statisticPlugin) Init(context.Context) error { return nil }
func (p *statisticPlugin) Close() error               { return nil }

func BuiltinRegistrations() []Registration {
	plugin := newStatisticPlugin()
	meta := PluginMeta{
		Name:        "statistic_plugin",
		Version:     "v1",
		Description: "Track only the latest turn metrics and log on turn end",
		Priority:    100,
		Enabled:     true,
		Kind:        PluginKindObserver,
		Stage:       EventChatMetric,
	}

	registrations := []Registration{{
		Meta:      meta,
		Lifecycle: plugin,
		Register: func(hub *Hub, meta PluginMeta) error {
			return hub.RegisterObserver(EventChatMetric, meta, plugin.onMetric)
		},
	}}

	// NATS bridge plugin is registered dynamically based on config
	// See RegisterNatsBridgePlugin() for the registration logic

	return registrations
}

func (p *statisticPlugin) onMetric(ctx Context, payload any) {
	data, ok := payload.(MetricData)
	if !ok || ctx.SessionID == "" {
		return
	}

	p.mu.Lock()
	nowTs := time.Now().UnixMilli()
	p.lastSeen[ctx.SessionID] = nowTs
	if p.cleanupCounter++; p.cleanupCounter%p.cleanupThreshold == 0 {
		p.cleanupStaleLocked(nowTs)
	}

	tm := p.getTurnForStageLocked(ctx.SessionID, data.Stage)
	var completed *turnMetric
	if tm != nil {
		completed = p.applyMetricLocked(ctx.SessionID, tm, data)
	}
	p.mu.Unlock()

	if completed != nil {
		p.logTurnMetric(ctx.SessionID, completed)
	}
}

func (p *statisticPlugin) getTurnForStageLocked(sessionID string, stage MetricStage) *turnMetric {
	if stage == MetricTurnStart {
		if tm, ok := p.current[sessionID]; ok && canReuseForLateTurnStart(tm) {
			return tm
		}
		return p.startNewTurnLocked(sessionID)
	}

	tm, ok := p.current[sessionID]
	if ok {
		return tm
	}
	if canStartTurnWithoutTurnStart(stage) {
		return p.startNewTurnLocked(sessionID)
	}
	return nil
}

func canReuseForLateTurnStart(tm *turnMetric) bool {
	if tm == nil {
		return false
	}
	return tm.turnStartTs == 0 && tm.llmStartTs == 0 && tm.ttsStartTs == 0 && tm.ttsStopTs == 0
}

func canStartTurnWithoutTurnStart(stage MetricStage) bool {
	switch stage {
	case MetricAsrFirstText, MetricAsrFinalText, MetricLlmStart, MetricTtsStart:
		return true
	default:
		return false
	}
}

func (p *statisticPlugin) startNewTurnLocked(sessionID string) *turnMetric {
	newTurnID := p.nextTurn[sessionID] + 1
	p.nextTurn[sessionID] = newTurnID

	tm := &turnMetric{turnID: newTurnID}
	p.current[sessionID] = tm
	return tm
}

func (p *statisticPlugin) applyMetricLocked(sessionID string, tm *turnMetric, data MetricData) *turnMetric {
	if data.Stage != MetricTurnStart && tm.turnStartTs > 0 && data.Ts > 0 && data.Ts < tm.turnStartTs {
		return nil
	}

	switch data.Stage {
	case MetricTurnStart:
		if tm.turnStartTs == 0 || (data.Ts > 0 && data.Ts < tm.turnStartTs) {
			tm.turnStartTs = data.Ts
		}
	case MetricTurnEnd:
		if tm.turnEndTs == 0 {
			tm.turnEndTs = data.Ts
		}
		snapshot := *tm
		delete(p.current, sessionID)
		return &snapshot
	case MetricAsrFirstText:
		if tm.asrFirstTextTs == 0 {
			tm.asrFirstTextTs = data.Ts
		}
	case MetricAsrFinalText:
		if tm.asrFinalTextTs == 0 {
			tm.asrFinalTextTs = data.Ts
		}
	case MetricLlmStart:
		if tm.llmStartTs == 0 {
			tm.llmStartTs = data.Ts
		}
	case MetricLlmFirstToken:
		if tm.llmStartTs > 0 && tm.llmFirstTokenTs == 0 {
			tm.llmFirstTokenTs = data.Ts
		}
	case MetricLlmEnd:
		if tm.llmStartTs > 0 && tm.llmEndTs == 0 {
			tm.llmEndTs = data.Ts
		}
	case MetricTtsStart:
		if tm.ttsStartTs == 0 {
			tm.ttsStartTs = data.Ts
		}
	case MetricTtsFirstFrame:
		if tm.ttsStartTs > 0 && tm.ttsFirstFrameTs == 0 {
			tm.ttsFirstFrameTs = data.Ts
		}
	case MetricTtsStop:
		if tm.ttsStartTs > 0 && tm.ttsStopTs == 0 {
			tm.ttsStopTs = data.Ts
		}
	}
	return nil
}

func calcDelta(start, end int64) int64 {
	if start <= 0 || end <= 0 || end < start {
		return 0
	}
	return end - start
}

func (p *statisticPlugin) logTurnMetric(sessionID string, tm *turnMetric) {
	e2eTotalEndTs := tm.turnEndTs
	if e2eTotalEndTs == 0 {
		e2eTotalEndTs = tm.ttsStopTs
	}

	log.Infof(
		"metric turn=%d session=%s asr_first=%dms asr_final=%dms llm_first=%dms llm_total=%dms tts_first=%dms tts_total=%dms e2e_first=%dms e2e_total=%dms",
		tm.turnID,
		sessionID,
		calcDelta(tm.turnStartTs, tm.asrFirstTextTs),
		calcDelta(tm.asrFirstTextTs, tm.asrFinalTextTs),
		calcDelta(tm.llmStartTs, tm.llmFirstTokenTs),
		calcDelta(tm.llmStartTs, tm.llmEndTs),
		calcDelta(tm.ttsStartTs, tm.ttsFirstFrameTs),
		calcDelta(tm.ttsStartTs, tm.ttsStopTs),
		calcDelta(tm.turnStartTs, tm.ttsFirstFrameTs),
		calcDelta(tm.turnStartTs, e2eTotalEndTs),
	)
}

func (p *statisticPlugin) cleanupStaleLocked(nowTs int64) {
	const ttl = int64(2 * 60 * 1000)

	for sessionID, lastSeenTs := range p.lastSeen {
		if nowTs-lastSeenTs <= ttl {
			continue
		}
		delete(p.lastSeen, sessionID)
		delete(p.current, sessionID)
		delete(p.nextTurn, sessionID)
	}
}
