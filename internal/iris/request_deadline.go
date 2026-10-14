package iris

import "time"

func requestDuration(cfg config, req request) time.Duration {
	if seconds, ok := messageWait(req); ok {
		return time.Duration(seconds+5) * time.Second
	}

	return time.Duration(cfg.settings.RequestTimeoutSeconds) * time.Second
}

func waitsForMessages(req request) bool {
	seconds, ok := messageWait(req)

	return ok && seconds > 0
}

func messageWait(req request) (int, bool) {
	var seconds int

	switch req.Operation {
	case "receive":
		var params receiveParams
		if err := decodeJSON(req.Params, &params); err != nil {
			return 0, false
		}

		seconds = params.WaitSeconds
	case "bridge.receive":
		var params bridgeReceiveParams
		if err := decodeJSON(req.Params, &params); err != nil {
			return 0, false
		}

		seconds = params.WaitSeconds
	default:
		return 0, false
	}

	return seconds, seconds >= 0 && seconds <= 60
}
