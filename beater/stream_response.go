package beater

type streamResponse struct {
	Errors   map[int]map[string]uint `json:"errors"`
	Accepted uint                    `json:"accepted"`
	Invalid  uint                    `json:"invalid"`
	Dropped  uint                    `json:"dropped"`
}

func (s *streamResponse) addErrorCount(serverResponse serverResponse, count int) {
	if s.Errors == nil {
		s.Errors = make(map[int]map[string]uint)
	}

	errorMsgs, ok := s.Errors[serverResponse.code]
	if !ok {
		s.Errors[serverResponse.code] = make(map[string]uint)
		errorMsgs = s.Errors[serverResponse.code]
	}
	errorMsgs[serverResponse.err.Error()] += uint(count)
}

func (s *streamResponse) addError(serverResponse serverResponse) {
	s.addErrorCount(serverResponse, 1)
}
