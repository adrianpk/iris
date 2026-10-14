package iris

import "time"

func requestDuration(cfg config, req request) time.Duration {
	if req.Operation == "receive" {
		var params receiveParams
		if err := decodeJSON(req.Params, &params); err == nil && params.WaitSeconds >= 0 && params.WaitSeconds <= 60 {
			return time.Duration(params.WaitSeconds+5) * time.Second
		}
	}

	return time.Duration(cfg.settings.RequestTimeoutSeconds) * time.Second
}

func waitsForMessages(req request) bool {
	if req.Operation != "receive" {
		return false
	}

	var params receiveParams

	return decodeJSON(req.Params, &params) == nil && params.WaitSeconds > 0 && params.WaitSeconds <= 60
}
